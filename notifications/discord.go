package notifications

import (
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
}

func (d *DiscordWebhook) Connect() bool {
	regex, _ := regexp.Compile("^https://discord.com/api/webhooks/([0-9]{18,20})/([0-9a-zA-Z_-]+)$")
	matches := regex.FindStringSubmatch(d.URL)
	if matches != nil {
		if len(matches) == 3 {
			id, _ := strconv.ParseUint(matches[1], 10, 64)
			client := webhook.New(discordsnowflake.ID(id), matches[2])
			d.Client = &client
			return true
		} else {
			return false
		}
	} else {
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
			client := *d.Client
			_, err := client.CreateEmbeds([]discord.Embed{embed})
			return err == nil
		}
	}
	return false
}
