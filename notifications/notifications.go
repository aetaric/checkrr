package notifications

import (
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Notification interface {
	Notify(string, string, string, string) bool
}

type Notifications struct {
	EnabledServices []Notification
	config          *koanf.Koanf
	Log             *logging.Log
	Localizer       *i18n.Localizer
}

func (n Notifications) Notify(title string, description string, notifType string, path string) {
	for _, service := range n.EnabledServices {
		service.Notify(title, description, notifType, path)
	}
}

func (n *Notifications) Connect() {
	if len(n.config.Cut("discord").Keys()) != 0 {
		discord := DiscordWebhook{Log: n.Log, Localizer: n.Localizer}
		discord.FromConfig(*n.config.Cut("discord"))
		discordConnected := discord.Connect()

		if discordConnected {
			n.EnabledServices = append(n.EnabledServices, discord)
		}
	}

	if len(n.config.Cut("healthchecks").Keys()) != 0 {
		healthcheck := Healthchecks{}
		healthcheck.FromConfig(*n.config.Cut("healthchecks"))
		healthcheckConnected := healthcheck.Connect()
		if healthcheckConnected {
			n.EnabledServices = append(n.EnabledServices, healthcheck)
		}
	}

	if len(n.config.Cut("telegram").Keys()) != 0 {
		telegram := Telegram{Log: n.Log, Localizer: n.Localizer}
		telegram.FromConfig(*n.config.Cut("telegram"))
		telegramConnected := telegram.Connect()
		if telegramConnected {
			n.EnabledServices = append(n.EnabledServices, telegram)
		}
	}

	if len(n.config.Cut("webhook").Keys()) != 0 {
		webhook := Notifywebhook{Log: n.Log}
		webhook.FromConfig(*n.config.Cut("webhook"))
		webhookConnected := webhook.Connect()
		if webhookConnected {
			n.EnabledServices = append(n.EnabledServices, webhook)
		}
	}

	if len(n.config.Cut("pushbullet").Keys()) != 0 {
		pushbullet := Pushbullet{Log: n.Log, Localizer: n.Localizer}
		pushbullet.FromConfig(*n.config.Cut("pushbullet"))
		pushbulletConnected := pushbullet.Connect()
		if pushbulletConnected {
			n.EnabledServices = append(n.EnabledServices, pushbullet)
		}
	}

	if len(n.config.Cut("pushover").Keys()) != 0 {
		pushover := Pushover{Log: n.Log, Localizer: n.Localizer}
		pushover.FromConfig(*n.config.Cut("pushover"))
		pushoverConnected := pushover.Connect()
		if pushoverConnected {
			n.EnabledServices = append(n.EnabledServices, pushover)
		}
	}

	if len(n.config.Cut("gotify").Keys()) != 0 {
		gotify := GotifyNotifs{Log: n.Log, Localizer: n.Localizer}
		gotify.FromConfig(*n.config.Cut("gotify"))
		gotifyConnected := gotify.Connect()
		if gotifyConnected {
			n.EnabledServices = append(n.EnabledServices, gotify)
		}
	}

	if len(n.config.Cut("splunk").Keys()) != 0 {
		splunk := SplunkHEC{Log: n.Log, Localizer: n.Localizer}
		splunk.FromConfig(*n.config.Cut("splunk"))
		splunkConnected := splunk.Connect()
		if splunkConnected {
			n.EnabledServices = append(n.EnabledServices, splunk)
		}
	}

	if len(n.config.Cut("ntfy").Keys()) != 0 {
		ntfy := NtfyNotifs{Log: n.Log, Localizer: n.Localizer}
		ntfy.FromConfig(*n.config.Cut("ntfy"))
		ntfyConnected := ntfy.Connect()
		if ntfyConnected {
			n.EnabledServices = append(n.EnabledServices, ntfy)
		}
	}

	if len(n.config.Cut("smtp").Keys()) != 0 {
		smtp := SMTPNotifs{Log: n.Log, Localizer: n.Localizer}
		smtp.FromConfig(*n.config.Cut("smtp"))
		smtpConnected := smtp.Connect()
		if smtpConnected {
			n.EnabledServices = append(n.EnabledServices, smtp)
		}
	}
}

func (n *Notifications) FromConfig(c *koanf.Koanf) {
	n.config = c
}
