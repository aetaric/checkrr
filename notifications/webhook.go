package notifications

import (
	"bytes"
	"encoding/json"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"net/http"
)

type Notifywebhook struct {
	url           string
	config        koanf.Koanf
	AllowedNotifs []string
	Log           *logging.Log
}

type payload struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

func (n *Notifywebhook) FromConfig(config koanf.Koanf) {
	n.config = config
	n.url = config.String("url")
	n.AllowedNotifs = config.Strings("notificationtypes")
}

func (n *Notifywebhook) Connect() bool {
	if n.url != "" {
		return true
	} else {
		return false
	}
}

func (n Notifywebhook) Notify(title string, description string, notifType string, path string) bool {
	var allowed bool = false
	for _, notif := range n.AllowedNotifs {
		if notif == notifType {
			allowed = true
		}
	}
	if allowed {
		data := payload{Type: notifType, Path: path}
		payloadBytes, err := json.Marshal(data)
		if err != nil {
			n.Log.Error(err.Error())
		}
		body := bytes.NewReader(payloadBytes)

		req, err := http.NewRequest("POST", n.url, body)
		if err != nil {
			n.Log.Error(err.Error())
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			n.Log.Error(err.Error())
		}
		defer resp.Body.Close()
		return true
	}
	return false
}
