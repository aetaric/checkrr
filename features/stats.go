package features

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"net/http"
	"os"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

type Stats struct {
	influxdb1         influxdb2.Client     `json:"-"`
	writeAPI1         api.WriteAPIBlocking `json:"-"`
	influxdb2         influxdb2.Client     `json:"-"`
	writeAPI2         api.WriteAPIBlocking `json:"-"`
	config            koanf.Koanf          `json:"-"`
	Log               logging.Log          `json:"-"`
	splunk            Splunk               `json:"-"`
	splunkConfigured  bool                 `json:"-"`
	SonarrSubmissions uint64               `json:"sonarrSubmissions"`
	RadarrSubmissions uint64               `json:"radarrSubmissions"`
	LidarrSubmissions uint64               `json:"lidarrSubmissions"`
	FilesChecked      uint64               `json:"filesChecked"`
	HashMatches       uint64               `json:"hashMatches"`
	HashMismatches    uint64               `json:"hashMismatches"`
	VideoFiles        uint64               `json:"videoFiles"`
	AudioFiles        uint64               `json:"audioFiles"`
	UnknownFileCount  uint64               `json:"unknownFileCount"`
	NonVideo          uint64               `json:"nonVideo"`
	Running           bool                 `json:"running"`
	startTime         time.Time            `json:"-"`
	endTime           time.Time            `json:"-"`
	Diff              time.Duration        `json:"timeDiff"`
	DB                *bolt.DB             `json:"-"`
}

type SplunkStats struct {
	Fields *SplunkFields `json:"fields"`
	Time   int64         `json:"time"`
	Event  string        `json:"event"`
}

type SplunkFields struct {
	SonarrSubmissions uint64 `json:"metric_name:checkrr.sonarrSubmissions"`
	RadarrSubmissions uint64 `json:"metric_name:checkrr.radarrSubmissions"`
	LidarrSubmissions uint64 `json:"metric_name:checkrr.lidarrSubmissions"`
	FilesChecked      uint64 `json:"metric_name:checkrr.filesChecked"`
	HashMatches       uint64 `json:"metric_name:checkrr.hashMatches"`
	HashMismatches    uint64 `json:"metric_name:checkrr.hashMismatches"`
	VideoFiles        uint64 `json:"metric_name:checkrr.videoFiles"`
	AudioFiles        uint64 `json:"metric_name:checkrr.audioFiles"`
	UnknownFileCount  uint64 `json:"metric_name:checkrr.unknownFileCount"`
	NonVideo          uint64 `json:"metric_name:checkrr.nonVideo"`
}

type Splunk struct {
	address string
	token   string
}

func (s *Stats) FromConfig(config koanf.Koanf) {
	s.config = config
	if len(config.Cut("influxdb1").Keys()) != 0 {
		influx := config.Cut("influxdb1")

		var token string
		if influx.String("user") != "" {
			token = fmt.Sprintf("%s:%s", influx.String("user"), influx.String("pass"))
		} else {
			token = ""
		}

		s.influxdb1 = influxdb2.NewClient(influx.String("url"), token)
		s.writeAPI1 = s.influxdb1.WriteAPIBlocking("", influx.String("bucket"))
		s.writeAPI1.EnableBatching()
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 1.x")
	}
	if len(config.Cut("influxdb2").Keys()) != 0 {
		influx := config.Cut("influxdb2")
		s.influxdb2 = influxdb2.NewClient(influx.String("url"), influx.String("token"))
		s.writeAPI2 = s.influxdb2.WriteAPIBlocking(influx.String("org"), influx.String("bucket"))
		s.writeAPI2.EnableBatching()
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 2.x")
	}
	if len(config.Cut("splunk").Keys()) != 0 {
		splunk := config.Cut("splunk")
		s.splunk = Splunk{address: splunk.String("address"), token: splunk.String("token")}
		s.splunkConfigured = true
		s.Log.WithFields(log.Fields{"startup": true, "splunk stats": "enabled"}).Info("Sending stats data to Splunk")
	}
}

func (s *Stats) Start() {
	s.startTime = time.Now()
	s.Running = true
	// Update stats DB
	err := s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		json, er := json.Marshal(s)
		if er != nil {
			return er
		}
		err := b.Put([]byte("current-stats"), json)
		return err
	})
	if err != nil {
		s.Log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
}

func (s *Stats) Stop() {
	s.endTime = time.Now()
	s.Diff = s.endTime.Sub(s.startTime)
	s.Running = false
	// Update stats DB
	err := s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		json, er := json.Marshal(s)
		if er != nil {
			return er
		}
		err := b.Put([]byte("current-stats"), json)
		return err
	})
	if err != nil {
		s.Log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
	err = s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		json, er := json.Marshal(s)
		if er != nil {
			return er
		}
		now := time.Now().UTC()
		err := b.Put([]byte(now.Format(time.RFC3339)), json)
		return err
	})
	if err != nil {
		s.Log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
}

func (s *Stats) Render() {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendRows([]table.Row{
		{"Files Checked", s.FilesChecked},
		{"Hash Matches", s.HashMatches},
		{"Hashes Mismatched", s.HashMismatches},
		{"Submitted to Sonarr", s.SonarrSubmissions},
		{"Submitted to Radarr", s.RadarrSubmissions},
		{"Submitted to Lidarr", s.LidarrSubmissions},
		{"Video Files", s.VideoFiles},
		{"Audio Files", s.AudioFiles},
		{"Text or Other Files", s.NonVideo},
		{"Unknown Files", s.UnknownFileCount},
		{"Elapsed Time", s.Diff},
	})
	t.Render()
}

func (s *Stats) Write(field string, count uint64) {
	// Send to influxdb if enabled
	if s.writeAPI1 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, float64(count)).
			SetTime(time.Now())
		err := s.writeAPI1.WritePoint(context.Background(), p)
		if err != nil {
			s.Log.Error(err.Error())
		}
	}
	if s.writeAPI2 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, float64(count)).
			SetTime(time.Now())
		s.writeAPI2.WritePoint(context.Background(), p)
	}
	// Send to splunk if configured
	if s.splunkConfigured {
		t := time.Now().Unix()
		splunkfields := SplunkFields{FilesChecked: s.FilesChecked, HashMatches: s.HashMatches, HashMismatches: s.HashMismatches,
			SonarrSubmissions: s.SonarrSubmissions, RadarrSubmissions: s.RadarrSubmissions, LidarrSubmissions: s.LidarrSubmissions,
			VideoFiles: s.VideoFiles, NonVideo: s.NonVideo, AudioFiles: s.AudioFiles, UnknownFileCount: s.UnknownFileCount}
		splunkstats := SplunkStats{Event: "metric", Time: t, Fields: &splunkfields}
		go func(splunkstats SplunkStats) {
			client := &http.Client{}
			j, _ := json.Marshal(splunkstats)
			var data = strings.NewReader(string(j))
			req, err := http.NewRequest("POST", s.splunk.address, data)
			if err != nil {
				s.Log.Warn(err)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", s.splunk.token))
			resp, err := client.Do(req)
			if err != nil {
				s.Log.Warn(err)
			}
			if resp != nil && resp.StatusCode != 200 {
				s.Log.Warnf("Recieved %d status code from Splunk", resp.StatusCode)
				defer resp.Body.Close()
			}
		}(splunkstats)
	}
	// Update stats DB
	err := s.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		json, er := json.Marshal(s)
		if er != nil {
			return er
		}
		err := b.Put([]byte("current-stats"), json)
		return err
	})
	if err != nil {
		s.Log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
}
