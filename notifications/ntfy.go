package notifications

import (
	"encoding/base64"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

type NtfyNotifs struct {
	host          string
	topic         string
	token         string
	user          string
	pass          string
	AllowedNotifs []string
	Log           *logging.Log
	Localizer     *i18n.Localizer
}

func (n *NtfyNotifs) FromConfig(config koanf.Koanf) {
	n.host = config.String("host")
	n.topic = config.String("topic")
	token := config.String("token")
	n.AllowedNotifs = config.Strings("notificationtypes")
	if token == "" {
		n.user = config.String("user")
		n.pass = config.String("password")
	} else {
		n.token = config.String("token")
	}

	if n.token == "" && n.user == "" {
		message := n.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsNtfySetupError",
		})
		n.Log.WithFields(log.Fields{"Startup": true, "Ntfy Connected": false}).Error(message)
	}
}

func (n *NtfyNotifs) Connect() bool {
	// stub method. ntfy is http only calls
	return true
}

func (n NtfyNotifs) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range n.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		req, err := http.NewRequest("POST", fmt.Sprintf("https://%s/%s", n.host, n.token), strings.NewReader(fmt.Sprintf("%s: %s", description, path)))
		if err != nil {
			message := n.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsNtfySendError",
				TemplateData: map[string]interface{}{
					"Error": err.Error(),
				},
			})
			n.Log.WithFields(log.Fields{"Notifications": "Ntfy"}).Error(message)
			return false
		} else {
			if n.user != "" {
				formatted := fmt.Sprintf("%s:%s", n.user, n.pass)
				authHeader := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(formatted)))
				req.Header.Set("Authorization", authHeader)
			} else if n.token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.token))
			}

			req.Header.Set("Title", title)
			req.Header.Set("Tags", notifType)
			_, err = http.DefaultClient.Do(req)
			if err != nil {
				message := n.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "NotificationsNtfySendError",
					TemplateData: map[string]interface{}{
						"Error": err.Error(),
					},
				})
				n.Log.WithFields(log.Fields{"Notifications": "Ntfy"}).Error(message)
				return false
			}
			return true
		}
	} else {
		return false
	}
}
