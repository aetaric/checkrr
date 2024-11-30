package notifications

import (
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

type Telegram struct {
	config        koanf.Koanf
	AllowedNotifs []string
	apiToken      string
	username      string
	chatid        int64
	bot           *tgbotapi.BotAPI
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (t Telegram) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range t.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		message := tgbotapi.NewMessage(t.chatid, description)
		t.bot.Send(message)
		return true
	}
	return false
}

func (t *Telegram) Connect() bool {
	var err error

	err = tgbotapi.SetLogger(t.Log)
	if err != nil {
		return false
	}

	t.bot, err = tgbotapi.NewBotAPI(t.apiToken)
	if err != nil {
		message := t.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsTelegramError",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		t.Log.WithFields(log.Fields{"Error": err, "Username": t.username, "Token": t.apiToken}).Warn(message)
		return false
	}

	updates, err := t.bot.GetUpdates(tgbotapi.UpdateConfig{})
	if err != nil {
		message := t.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsTelegramError",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		t.Log.WithFields(log.Fields{"Error": err, "Username": t.username, "Token": t.apiToken}).Warn(message)
		return false
	}
	if t.chatid == 0 {
		for _, update := range updates {
			message := t.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsTelegramDebugUser",
				TemplateData: map[string]interface{}{
					"Username": update.SentFrom().UserName,
				},
			})
			t.Log.Debug(message)
			if fmt.Sprintf("@%s", update.SentFrom().UserName) == t.username {
				t.chatid = update.FromChat().ID
				message = t.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "NotificationsTelegramDebugUser",
					TemplateData: map[string]interface{}{
						"ChatID": t.chatid,
					},
				})
				t.Log.Infof(message)
				break
			}
		}
	}
	message := t.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsTelegramConnect",
	})
	t.Log.Info(message)
	return true
}

func (t *Telegram) FromConfig(config koanf.Koanf) {
	t.config = config
	t.apiToken = config.String("apitoken")
	t.username = config.String("username")
	t.chatid = config.Int64("chatid")
	t.AllowedNotifs = config.Strings("notificationtypes")
}
