package notifications

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Notification interface {
	Notify(string, string, string, string) bool
}

type Notifications struct {
	EnabledServices []Notification
	config          viper.Viper
	Log             log.Logger
}

func (n Notifications) Notify(title string, description string, notifType string, path string) {
	for _, service := range n.EnabledServices {
		service.Notify(title, description, notifType, path)
	}
}

func (n *Notifications) Connect() {
	if n.config.Sub("discord") != nil {
		discord := DiscordWebhook{}
		discord.FromConfig(*n.config.Sub("discord"))
		discordConnected, discordMessage := discord.Connect()
		n.Log.WithFields(log.Fields{"Startup": true, "Discord Connected": discordConnected}).Info(discordMessage)
		if discordConnected {
			n.EnabledServices = append(n.EnabledServices, discord)
		}
	} else {
		n.Log.WithFields(log.Fields{"Startup": true, "Discord Connected": false}).Info("No Discord Webhook URL provided.")
	}

	if n.config.Sub("healthchecks") != nil {
		healthcheck := Healthchecks{}
		healthcheck.FromConfig(*n.config.Sub("healthcheck"))
		n.EnabledServices = append(n.EnabledServices, healthcheck)
	}

	if n.config.Sub("telegram") != nil {
		telegram := Telegram{Log: *log.StandardLogger()}
		telegram.FromConfig(*n.config.Sub("telegram"))
		n.EnabledServices = append(n.EnabledServices, telegram)
	}
}

func (n *Notifications) FromConfig(c viper.Viper) {
	n.config = c
}
