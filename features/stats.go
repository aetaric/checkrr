package features

import (
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

type Stats struct {
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
