package notifications

import (
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
		tgbotapi.NewMessageToChannel(t.username, description)
		return true
	}
	return false
}

func (t *Telegram) Connect() bool {
	var err error
	t.bot, err = tgbotapi.NewBotAPI(t.apiToken)
	if err != nil {
		log.Error(err)
		t.Log.WithFields(log.Fields{"Error": err, "Username": t.username, "Token": t.apiToken}).Warn("Error connecting to Telegram")
	}
	t.Log.Info("Connected to Telegram")
	return false
}

func (t *Telegram) FromConfig(config viper.Viper) {
	t.config = config
	t.apiToken = config.GetString("apitoken")
	t.username = config.GetString("username")
	t.AllowedNotifs = config.GetStringSlice("notificationtypes")
}
