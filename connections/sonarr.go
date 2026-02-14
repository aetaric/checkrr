package connections

import (
	"fmt"
	"strings"

	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"golift.io/starr"
	"golift.io/starr/sonarr"
)

type Sonarr struct {
	config    *starr.Config
	server    *sonarr.Sonarr
	Process   bool
	ApiKey    string
	Address   string
	Port      int
	BaseURL   string
	SSL       bool
	pathMaps  map[string]string
	Log       *logging.Log
	Localizer *i18n.Localizer
}

func (s *Sonarr) FromConfig(conf *koanf.Koanf) {
	if conf != nil {
		s.Address = conf.String("address")
		s.Process = conf.Bool("process")
		s.ApiKey = conf.String("apikey")
		s.Port = conf.Int("port")
		s.BaseURL = conf.String("baseurl")
		s.pathMaps = conf.StringMap("mappings")
		s.SSL = conf.Bool("ssl")
		s.Log.Debugf("Sonarr Path Maps: %v", s.pathMaps)
	} else {
		s.Process = false
	}
}

func (s *Sonarr) MatchPath(path string) bool {
	sonarrFolders, _ := s.server.GetRootFolders()
	for _, folder := range sonarrFolders {
		message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ArrErrorDeleting",
			TemplateData: map[string]interface{}{
				"Service":    "sonarr",
				"RootFolder": folder.Path,
				"File":       path,
			},
		})
		s.Log.Debug(message)
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
					err := s.server.DeleteEpisodeFile(file.ID)
					if err != nil {
						message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "ArrErrorDeleting",
							TemplateData: map[string]interface{}{
								"Type":  "episode",
								"ID":    file.ID,
								"Error": err.Error(),
							},
						})
						s.Log.Error(message)
					}
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
				message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ArrConnected",
					TemplateData: map[string]interface{}{
						"Service": "Sonarr",
					},
				})
				return true, message
			}
		} else {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrMissingArgs",
				TemplateData: map[string]interface{}{
					"Service": "Sonarr",
				},
			})
			return false, message
		}
	}
	message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "ArrNoOp",
		TemplateData: map[string]interface{}{
			"Service": "Sonarr",
		},
	})
	return false, message
}

func (s Sonarr) translatePath(path string) string {
	keys := make([]string, 0, len(s.pathMaps))
	for k := range s.pathMaps {
		keys = append(keys, k)
	}
	for _, key := range keys {
		if strings.Contains(path, s.pathMaps[key]) {
			message := s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapKey",
				TemplateData: map[string]interface{}{
					"Key": key,
				},
			})
			s.Log.Debug(message)
			message = s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapValue",
				TemplateData: map[string]interface{}{
					"Value": s.pathMaps[key],
				},
			})
			s.Log.Debug(message)
			message = s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapOriginal",
				TemplateData: map[string]interface{}{
					"Path": path,
				},
			})
			s.Log.Debug(message)
			replaced := strings.Replace(path, s.pathMaps[key], key, -1)
			message = s.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapNew",
				TemplateData: map[string]interface{}{
					"Path": replaced,
				},
			})
			s.Log.Debug(message)
			return replaced
		}
	}
	return path
}
