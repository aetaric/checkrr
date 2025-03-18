package notifications

import (
	"net/http"
	"net/url"

	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/gotify/go-api-client/v2/auth"
	"github.com/gotify/go-api-client/v2/client"
	"github.com/gotify/go-api-client/v2/client/message"
	"github.com/gotify/go-api-client/v2/gotify"
	"github.com/gotify/go-api-client/v2/models"
)

type GotifyNotifs struct {
	URL           string
	Client        *client.GotifyREST
	AuthToken     string
	Connected     bool
	AllowedNotifs []string
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (d *GotifyNotifs) FromConfig(config koanf.Koanf) {
	d.URL = config.String("url")
	d.AllowedNotifs = config.Strings("notificationtypes")
	d.AuthToken = config.String("authtoken")
}

func (d *GotifyNotifs) Connect() bool {
	myURL, _ := url.Parse(d.URL)
	newClient := gotify.NewClient(myURL, &http.Client{})
	versionResponse, err := newClient.Version.GetVersion(nil)

	if err != nil {
		failureMessage := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsGotifyConnect",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		d.Log.Warn(failureMessage)
		return false
	}
	version := versionResponse.Payload
	d.Client = newClient
	d.Connected = true
	connectMessage := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsGotifyConnect",
		TemplateData: map[string]interface{}{
			"Version": version,
		},
	})
	d.Log.Info(connectMessage)
	return true
}

func (d GotifyNotifs) Notify(title string, description string, notifType string, path string) bool {
	if d.Connected {
		var allowed bool = false
		for _, notif := range d.AllowedNotifs {
			if notif == notifType {
				allowed = true
			}
		}
		if allowed {
			params := message.NewCreateMessageParams()
			params.Body = &models.MessageExternal{
				Title:    title,
				Message:  description,
				Priority: 5,
			}
			_, err := d.Client.Message.CreateMessage(params, auth.TokenAuth(d.AuthToken))

			return err == nil
		}
	}
	return false
}
