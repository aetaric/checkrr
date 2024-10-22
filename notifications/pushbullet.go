package notifications

import (
	"github.com/aetaric/checkrr/logging"
	"github.com/spf13/viper"
	"github.com/xconstruct/go-pushbullet"
)

type Pushbullet struct {
	config        viper.Viper
	AllowedNotifs []string
	apiToken      string
	devices       []string
	bot           *pushbullet.Client
	Log           *logging.Log
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
		return false
	} else {
		return true
	}
}

func (p *Pushbullet) FromConfig(config viper.Viper) {
	p.config = config
	p.apiToken = config.GetString("apitoken")
	p.devices = config.GetStringSlice("devices")
	p.AllowedNotifs = config.GetStringSlice("notificationtypes")
}
