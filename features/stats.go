package features

import (
	"context"
	"fmt"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Stats struct {
	influxdb1           influxdb2.Client
	writeAPI1           api.WriteAPIBlocking
	influxdb2           influxdb2.Client
	writeAPI2           api.WriteAPIBlocking
	config              viper.Viper
	Log                 log.Logger
	SonarrSubmissions   uint64
	RadarrSubmissions   uint64
	LidarrSubmissions   uint64
	FilesChecked        uint64
	HashMatches         uint64
	HashMismatches      uint64
	VideoFiles          uint64
	AudioFiles          uint64
	UnknownFileCount    uint64
	UnknownFilesDeleted uint64
	NonVideo            uint64
	Running             bool
	startTime           time.Time
	endTime             time.Time
	Diff                time.Duration
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
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 1.x")
	}
	if config.Sub("influxdb2") != nil {
		influx := config.Sub("influxdb2")
		s.influxdb2 = influxdb2.NewClient(influx.GetString("url"), influx.GetString("token"))
		s.writeAPI2 = s.influxdb1.WriteAPIBlocking(influx.GetString("org"), influx.GetString("bucket"))
		s.Log.WithFields(log.Fields{"startup": true, "influxdb": "enabled"}).Info("Sending data to InfluxDB 2.x")
	}
}

func (s *Stats) Start() {
	s.startTime = time.Now()
	s.Running = true
}

func (s *Stats) Stop() {
	s.endTime = time.Now()
	s.Diff = s.endTime.Sub(s.startTime)
	s.Running = false
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
		{"Unknown File Deletes", s.UnknownFilesDeleted},
		{"Elapsed Time", s.Diff},
	})
	t.Render()
}

func (s *Stats) Write(field string, count float32) {
	if s.writeAPI1 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, count).
			SetTime(time.Now())
		s.writeAPI1.WritePoint(context.Background(), p)
	}
	if s.writeAPI2 != nil {
		p := influxdb2.NewPointWithMeasurement("checkrr").
			AddField(field, count).
			SetTime(time.Now())
		s.writeAPI2.WritePoint(context.Background(), p)
	}
}
