package notifications

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type NtfyNotifs struct {
	host          string
	topic         string
	token         string
	user          string
	pass          string
	AllowedNotifs []string
	Log           log.Logger
}

func (n *NtfyNotifs) FromConfig(config viper.Viper) {
	n.host = config.GetString("host")
	n.topic = config.GetString("topic")
	token := config.GetString("token")
	n.AllowedNotifs = config.GetStringSlice("notificationtypes")
	if token == "" {
		n.user = config.GetString("user")
		n.pass = config.GetString("password")
	} else {
		n.token = config.GetString("token")
	}

	if n.token == "" && n.user == "" {
		n.Log.WithFields(log.Fields{"Startup": true, "Ntfy Connected": false}).Error("Error connecting. Please either use a token or a user. Not both.")
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
			n.Log.WithFields(log.Fields{"Notifications": "Ntfy"}).Error(fmt.Sprintf("Error setting up the http request: %s", err))
		}
		if n.user != "" {
			formatted := fmt.Sprintf("%s:%s", n.user, n.pass)
			authHeader := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(formatted)))
			req.Header.Set("Authorization", authHeader)
		} else if n.token != "" {
			req.Header.Set("Authorization", fmt.Sprint("Bearer %s", n.token))
		}

		req.Header.Set("Title", title)
		req.Header.Set("Tags", notifType)
		http.DefaultClient.Do(req)
		return true
	} else {
		return false
	}
}
