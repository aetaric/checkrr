package features

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
)

type Stats struct {
	influxdb1         influxdb2.Client     `json:"-"`
	writeAPI1         api.WriteAPIBlocking `json:"-"`
	influxdb2         influxdb2.Client     `json:"-"`
	writeAPI2         api.WriteAPIBlocking `json:"-"`
	config            viper.Viper          `json:"-"`
	Log               log.Logger           `json:"-"`
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

func (s *Stats) FromConfig(config viper.Viper) {
	s.config = config
	if config.Sub("influxdb1") != nil {
		influx := config.Sub("influxdb1")

		var token string
		if influx.GetString("user") != "" {
			token = fmt.Sprintf("%s:%s", influx.GetString("user"), influx.GetString("pass"))
		} else {
			token = ""
		}

		s.influxdb1 = influxdb2.NewClient(influx.GetString("url"), token)
		s.writeAPI1 = s.influxdb1.WriteAPIBlocking("", influx.GetString("bucket"))
		s.writeAPI1.EnableBatching()
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 1.x")
	}
	if config.Sub("influxdb2") != nil {
		influx := config.Sub("influxdb2")
		s.influxdb2 = influxdb2.NewClient(influx.GetString("url"), influx.GetString("token"))
		org, _ := s.influxdb2.OrganizationsAPI().FindOrganizationByName(context.Background(), influx.GetString("org"))
		s.influxdb2.BucketsAPI().CreateBucketWithName(context.Background(), org, influx.GetString("bucket"))
		s.writeAPI2 = s.influxdb1.WriteAPIBlocking(influx.GetString("org"), influx.GetString("bucket"))
		s.writeAPI2.EnableBatching()
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 2.x")
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
		log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
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
		log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
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
		log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
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

func (s Stats) Write(field string, count uint64) {
	// Send to influxdb if enabled
	if s.writeAPI1 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, float64(count)).
			SetTime(time.Now())
		err := s.writeAPI1.WritePoint(context.Background(), p)
		if err != nil {
			log.Error(err.Error())
		}
	}
	if s.writeAPI2 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, float64(count)).
			SetTime(time.Now())
		s.writeAPI2.WritePoint(context.Background(), p)
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
		log.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
}
