package notifications

import (
	"github.com/aetaric/checkrr/logging"
	"github.com/gregdel/pushover"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Pushover struct {
	config        koanf.Koanf
	AllowedNotifs []string
	apiToken      string
	recipient     *pushover.Recipient
	bot           *pushover.Pushover
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (p Pushover) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range p.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		message := pushover.NewMessageWithTitle(description, title)
		_, err := p.bot.SendMessage(message, p.recipient)
		if err != nil {
			p.Log.Error(err.Error())
			return false
		}
		return true
	}
	return false
}

func (p *Pushover) Connect() bool {
	p.bot = pushover.New(p.apiToken)
	p.recipient = pushover.NewRecipient(p.config.String("recipient"))
	if p.bot != nil {
		message := p.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsPushOverConnect",
		})
		p.Log.Info(message)
		return true
	} else {
		message := p.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsPushOverError",
		})
		p.Log.Warn(message)
		return false
	}
}

func (p *Pushover) FromConfig(config koanf.Koanf) {
	p.config = config
	p.apiToken = config.String("apitoken")
	p.AllowedNotifs = config.Strings("notificationtypes")
}
