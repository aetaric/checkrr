package connections

import (
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"strings"

	"github.com/spf13/viper"
	"golift.io/starr"
	"golift.io/starr/sonarr"
)

type Sonarr struct {
	config   *starr.Config
	server   *sonarr.Sonarr
	Process  bool
	ApiKey   string
	Address  string
	Port     int
	BaseURL  string
	SSL      bool
	pathMaps map[string]string
	Log      *logging.Log
}

func (s *Sonarr) FromConfig(conf *viper.Viper) {
	if conf != nil {
		s.Address = conf.GetString("address")
		s.Process = conf.GetBool("process")
		s.ApiKey = conf.GetString("apikey")
		s.Port = conf.GetInt("port")
		s.BaseURL = conf.GetString("baseurl")
		s.pathMaps = conf.GetStringMapString("mappings")
		s.SSL = conf.GetBool("ssl")
		s.Log.Debugf("Sonarr Path Maps: %v", s.pathMaps)
	} else {
		s.Process = false
	}
}

func (s *Sonarr) MatchPath(path string) bool {
	sonarrFolders, _ := s.server.GetRootFolders()
	for _, folder := range sonarrFolders {
		if strings.Contains(s.translatePath(path), folder.Path) {
			return true
		}
	}
	return false
}

func (s *Sonarr) RemoveFile(path string) bool {
	var seriesID int64
	seriesList, _ := s.server.GetAllSeries()
	for _, series := range seriesList {
		if strings.Contains(s.translatePath(path), series.Path) {
			seriesID = series.ID
			files, _ := s.server.GetSeriesEpisodeFiles(seriesID)
			for _, file := range files {
				if file.Path == s.translatePath(path) {
					s.server.DeleteEpisodeFile(file.ID)
					s.server.SendCommand(&sonarr.CommandRequest{Name: "RescanSeries", SeriesID: seriesID})
					s.server.SendCommand(&sonarr.CommandRequest{Name: "SeriesSearch", SeriesID: seriesID})
					return true
				}
			}
			return false
		}
	}
	return false
}

func (s *Sonarr) Connect() (bool, string) {
	if s.Process {
		if s.ApiKey != "" {
			protocol := "http"
			if s.SSL {
				protocol = "https"
			}
			s.config = starr.New(s.ApiKey, fmt.Sprintf("%s://%s:%v%v", protocol, s.Address, s.Port, s.BaseURL), 0)
			s.server = sonarr.New(s.config)
			status, err := s.server.GetSystemStatus()
			if err != nil {
				return false, err.Error()
			}

			if status.Version != "" {
				return true, "Sonarr Connected."
			}
		} else {
			return false, "Missing Sonarr arguments"
		}
	}
	return false, "Sonarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)"
}

func (s Sonarr) translatePath(path string) string {
	keys := make([]string, 0, len(s.pathMaps))
	for k := range s.pathMaps {
		keys = append(keys, k)
	}
	for _, key := range keys {
		if strings.Contains(path, s.pathMaps[key]) {
			s.Log.Debugf("Key: %s", key)
			s.Log.Debugf("Value: %s", s.pathMaps[key])
			s.Log.Debugf("Original path: %s", path)
			replaced := strings.Replace(path, s.pathMaps[key], key, -1)
			s.Log.Debugf("New path: %s", replaced)
			return replaced
		}
	}
	return path
}
