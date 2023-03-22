package notifications

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Telegram struct {
	config        viper.Viper
	AllowedNotifs []string
	apiToken      string
	username      string
	bot           *tgbotapi.BotAPI
	Log           log.Logger
}

func (t Telegram) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range t.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		var chatid int64
		updates, err := t.bot.GetUpdates(tgbotapi.UpdateConfig{})
		if err != nil {
			return false
		}
		for _, update := range updates {
			t.Log.Debug(fmt.Sprintf("User: %s", update.SentFrom().UserName))
			if fmt.Sprintf("@%s", update.SentFrom().UserName) == t.username {
				chatid = update.FromChat().ID
				t.Log.Debug(fmt.Sprintf("chatid: %v", chatid))
				break
			}
		}
		message := tgbotapi.NewMessage(chatid, description)
		t.bot.Send(message)
		return true
	}
	return false
}

func (t *Telegram) Connect() bool {
	var err error
	tgbotapi.SetLogger(&t.Log)
	t.bot, err = tgbotapi.NewBotAPI(t.apiToken)
	if err != nil {
		log.Error(err)
		t.Log.WithFields(log.Fields{"Error": err, "Username": t.username, "Token": t.apiToken}).Warn("Error connecting to Telegram")
		return false
	}
	t.Log.Info("Connected to Telegram")
	return true
}

func (t *Telegram) FromConfig(config viper.Viper) {
	t.config = config
	t.apiToken = config.GetString("apitoken")
	t.username = config.GetString("username")
	t.AllowedNotifs = config.GetStringSlice("notificationtypes")
}
