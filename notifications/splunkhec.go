package notifications

import (
	"encoding/json"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type SplunkHEC struct {
	URL           string
	Token         string
	Connected     bool
	AllowedNotifs []string
	Log           *logging.Log
	Localizer     *i18n.Localizer
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

func (d *SplunkHEC) FromConfig(config koanf.Koanf) {
	d.URL = config.String("url")
	d.Token = config.String("token")
	d.AllowedNotifs = config.Strings("notificationtypes")
}

func (d *SplunkHEC) Connect() bool {
	if d.Token != "" && d.URL != "" {
		message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsSplunkHECConnect",
		})
		d.Log.Info(message)
		d.Connected = true
		return true
	} else {
		message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsSplunkHECError",
		})
		d.Log.Info(message)
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
					message := d.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "StatsSplunkError",
						TemplateData: map[string]interface{}{
							"Code": resp.StatusCode,
						},
					})
					log.Warn(message)
					defer func(Body io.ReadCloser) {
						err := Body.Close()
						if err != nil {
							d.Log.Error(err)
						}
					}(resp.Body)
				}
			}(splunkevent)
			return true
		}
	}
	return false
}
