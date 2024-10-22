package notifications

import (
	"encoding/json"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type SplunkHEC struct {
	URL           string
	Token         string
	Connected     bool
	AllowedNotifs []string
	Log           *logging.Log
}

type SplunkEventData struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type SplunkEvent struct {
	Event      *SplunkEventData `json:"event"`
	Time       int64            `json:"time"`
	SourceType string           `json:"sourcetype"`
}

func (d *SplunkHEC) FromConfig(config viper.Viper) {
	d.URL = config.GetString("url")
	d.Token = config.GetString("token")
	d.AllowedNotifs = config.GetStringSlice("notificationtypes")
}

func (d *SplunkHEC) Connect() bool {
	if d.Token != "" && d.URL != "" {
		d.Log.Info("Splunk HTTP Event Collector \"Connected\"")
		d.Connected = true
		return true
	} else {
		d.Log.Info("Splunk HTTP Event Collector Error")
		return false
	}
}

func (d SplunkHEC) Notify(title string, description string, notifType string, path string) bool {
	if d.Connected {
		var allowed bool = false
		for _, notif := range d.AllowedNotifs {
			if notif == notifType {
				allowed = true
			}
		}
		if allowed {
			t := time.Now().Unix()
			splunkeventdata := SplunkEventData{Type: notifType, Path: path}
			splunkevent := SplunkEvent{Event: &splunkeventdata, Time: t, SourceType: "_json"}
			go func(splunkevent SplunkEvent) {
				client := &http.Client{}
				j, _ := json.Marshal(splunkevent)
				var data = strings.NewReader(string(j))
				req, err := http.NewRequest("POST", d.URL, data)
				if err != nil {
					log.Warn(err)
				}
				req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", d.Token))
				resp, err := client.Do(req)
				if err != nil {
					log.Warn(err)
				}
				if resp != nil && resp.StatusCode != 200 {
					log.Warnf("Recieved %d status code from Splunk", resp.StatusCode)
					defer resp.Body.Close()
				}
			}(splunkevent)
			return true
		}
	}
	return false
}
