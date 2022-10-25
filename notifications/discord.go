package notifications

import (
	"regexp"
	"strconv"

	"github.com/disgoorg/disgo/discord"
	webhook "github.com/disgoorg/disgo/webhook"
	discordsnowflake "github.com/disgoorg/snowflake/v2"
	"github.com/sirupsen/logrus"
)

type DiscordWebhook struct {
	URL           string
	Log           logrus.Logger
	Client        webhook.Client
	Connected     bool
	AllowedNotifs []string
}

func (d DiscordWebhook) Connect() bool {
	regex, _ := regexp.Compile("^https://discord.com/api/webhooks/([0-9]{18,20})/([0-9a-zA-Z_-]+)$")
	matches := regex.FindStringSubmatch(d.URL)
	if matches != nil {
		if len(matches) == 3 {
			id, _ := strconv.ParseUint(matches[1], 10, 64)
			d.Client = webhook.New(discordsnowflake.ID(id), matches[2])
			d.Log.Info("Connected To Discord")
			return true
		} else {
			d.Log.Info("incorrect length")
			return false
		}
	} else {
		d.Log.Info("No match")
		return false
	}
}

func (d DiscordWebhook) Notify(title string, description string, notifType string) bool {
	if d.Connected {
		var allowed bool = false
		for _, notif := range d.AllowedNotifs {
			if notif == notifType {
				allowed = true
			}
		}
		if allowed {
			embed := discord.NewEmbedBuilder().SetDescriptionf(description).SetTitlef(title).Build()
			_, err := d.Client.CreateEmbeds([]discord.Embed{embed})
			return err == nil
		}
	}
	return false
}
