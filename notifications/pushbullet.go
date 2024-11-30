package notifications

import (
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/xconstruct/go-pushbullet"
)

type Pushbullet struct {
	config        koanf.Koanf
	AllowedNotifs []string
	apiToken      string
	devices       []string
	bot           *pushbullet.Client
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (p Pushbullet) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range p.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		for _, deviceName := range p.devices {
			device, err := p.bot.Device(deviceName)
			if err != nil {
				p.Log.Error(err.Error())
			}
			if device != nil {
				err = device.PushNote(title, description)
				if err != nil {
					p.Log.Error(err.Error())
				}
			}
		}
		return true
	}
	return false
}

func (p *Pushbullet) Connect() bool {
	p.bot = pushbullet.New(p.apiToken)
	_, err := p.bot.Me()
	if err != nil {
		message := p.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsPushBulletError",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		p.Log.Warn(message)
		return false
	} else {
		message := p.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsPushBulletConnect",
		})
		p.Log.Info(message)
		return true
	}
}

func (p *Pushbullet) FromConfig(config koanf.Koanf) {
	p.config = config
	p.apiToken = config.String("apitoken")
	p.devices = config.Strings("devices")
	p.AllowedNotifs = config.Strings("notificationtypes")
}
