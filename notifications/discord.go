package notifications

import (
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strconv"

	"github.com/disgoorg/disgo/discord"
	webhook "github.com/disgoorg/disgo/webhook"
	discordsnowflake "github.com/disgoorg/snowflake/v2"
)

type DiscordWebhook struct {
	URL           string
	Client        *webhook.Client
	Connected     bool
	AllowedNotifs []string
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (d *DiscordWebhook) FromConfig(config koanf.Koanf) {
	d.URL = config.String("URL")
	d.AllowedNotifs = config.Strings("notificationtypes")
}

func (d *DiscordWebhook) Connect() bool {
	regex, _ := regexp.Compile("^https://discord.com/api/webhooks/([0-9]{18,20})/([0-9a-zA-Z_-]+)$")
	matches := regex.FindStringSubmatch(d.URL)
	if matches != nil {
		if len(matches) == 3 {
			id, _ := strconv.ParseUint(matches[1], 10, 64)
			client := webhook.New(discordsnowflake.ID(id), matches[2])
			d.Client = &client
			d.Connected = true
			message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsDiscordConnect",
			})
			d.Log.WithFields(log.Fields{"Startup": true, "Discord Connected": true}).Info(message)
			return true
		} else {
			message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsDiscordFormat",
			})
			d.Log.WithFields(log.Fields{"Startup": true, "Discord Connected": false}).Warn(message)
			return false
		}
	} else {
		message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsDiscordFormat",
		})
		d.Log.WithFields(log.Fields{"Startup": true, "Discord Connected": false}).Warn(message)
		return false
	}
}

func (d DiscordWebhook) Notify(title string, description string, notifType string, path string) bool {
	if d.Connected {
		var allowed bool = false
		for _, notif := range d.AllowedNotifs {
			if notif == notifType {
				allowed = true
			}
		}
		if allowed {
			embed := discord.NewEmbedBuilder().SetDescriptionf(description).SetTitlef(title).Build()
			client := *d.Client
			_, err := client.CreateEmbeds([]discord.Embed{embed})
			return err == nil
		}
	}
	return false
}
