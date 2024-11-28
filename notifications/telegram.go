package notifications

import (
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"

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
		log.Error(err)
		t.Log.WithFields(log.Fields{"Error": err, "Username": t.username, "Token": t.apiToken}).Warn("Error connecting to Telegram")
		return false
	}

	updates, err := t.bot.GetUpdates(tgbotapi.UpdateConfig{})
	if err != nil {
		return false
	}
	if t.chatid == 0 {
		for _, update := range updates {
			t.Log.Debug(fmt.Sprintf("User: %s", update.SentFrom().UserName))
			if fmt.Sprintf("@%s", update.SentFrom().UserName) == t.username {
				t.chatid = update.FromChat().ID
				t.Log.Infof("Telegram chatid: %v", t.chatid)
				break
			}
		}
	}
	t.Log.Info("Connected to Telegram")
	return true
}

func (t *Telegram) FromConfig(config koanf.Koanf) {
	t.config = config
	t.apiToken = config.String("apitoken")
	t.username = config.String("username")
	t.chatid = config.Int64("chatid")
	t.AllowedNotifs = config.Strings("notificationtypes")
}
