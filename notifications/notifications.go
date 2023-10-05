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
	}

	if n.config.Sub("healthchecks") != nil {
		healthcheck := Healthchecks{}
		healthcheck.FromConfig(*n.config.Sub("healthchecks"))
		healthcheckConnected := healthcheck.Connect()
		if healthcheckConnected {
			n.EnabledServices = append(n.EnabledServices, healthcheck)
		}
	}

	if n.config.Sub("telegram") != nil {
		telegram := Telegram{Log: *log.StandardLogger()}
		telegram.FromConfig(*n.config.Sub("telegram"))
		telegramConnected := telegram.Connect()
		if telegramConnected {
			n.EnabledServices = append(n.EnabledServices, telegram)
		}
	}

	if n.config.Sub("webhook") != nil {
		webhook := Notifywebhook{Log: *log.StandardLogger()}
		webhook.FromConfig(*n.config.Sub("webhook"))
		webhookConnected := webhook.Connect()
		if webhookConnected {
			n.EnabledServices = append(n.EnabledServices, webhook)
		}
	}

	if n.config.Sub("pushbullet") != nil {
		pushbullet := Pushbullet{Log: *log.StandardLogger()}
		pushbullet.FromConfig(*n.config.Sub("pushbullet"))
		pushbulletConnected := pushbullet.Connect()
		if pushbulletConnected {
			n.EnabledServices = append(n.EnabledServices, pushbullet)
		}
	}

	if n.config.Sub("pushover") != nil {
		pushover := Pushover{Log: *log.StandardLogger()}
		pushover.FromConfig(*n.config.Sub("pushover"))
		pushoverConnected := pushover.Connect()
		if pushoverConnected {
			n.EnabledServices = append(n.EnabledServices, pushover)
		}
	}

	if n.config.Sub("gotify") != nil {
		gotify := GotifyNotifs{Log: *log.StandardLogger()}
		gotify.FromConfig(*n.config.Sub("gotify"))
		gotifyConnected := gotify.Connect()
		if gotifyConnected {
			n.EnabledServices = append(n.EnabledServices, gotify)
		}
	}

	if n.config.Sub("splunk") != nil {
		splunk := SplunkHEC{Log: *log.StandardLogger()}
		splunk.FromConfig(*n.config.Sub("splunk"))
		splunkConnected := splunk.Connect()
		if splunkConnected {
			n.EnabledServices = append(n.EnabledServices, splunk)
		}
	}
}

func (n *Notifications) FromConfig(c viper.Viper) {
	n.config = c
}
