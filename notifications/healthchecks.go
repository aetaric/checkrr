package notifications

import (
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Healthchecks struct {
	config        viper.Viper
	AllowedNotifs []string
	URL           string
}

func (h Healthchecks) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range h.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		var client = &http.Client{
			Timeout: 10 * time.Second,
		}

		var url string
		if notifType == "startrun" { // If we are starting up, we should use that endpoint
			url = h.URL + "/start"
			client.Head(url)
		} else if notifType == "endrun" { // pinging the URL will signal end
			url = h.URL
			client.Head(url)
		} else { // all other notif types are logs
			url = h.URL + "/log"
			reader := strings.NewReader(description)
			client.Post(url, "text/plain; charset=utf8", reader)
		}
		return true
	}
	return false
}

func (h *Healthchecks) FromConfig(config viper.Viper) {
	h.config = config
	h.URL = config.GetString("url")
	h.AllowedNotifs = config.GetStringSlice("notificationtypes")
}
