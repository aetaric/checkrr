package connections

import (
	"fmt"
	"strings"

	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"golift.io/starr"
	"golift.io/starr/lidarr"
)

type Lidarr struct {
	config    *starr.Config
	server    *lidarr.Lidarr
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

func (l *Lidarr) FromConfig(conf *koanf.Koanf) {
	if conf != nil {
		l.Address = conf.String("address")
		l.Process = conf.Bool("process")
		l.ApiKey = conf.String("apikey")
		l.Port = conf.Int("port")
		l.BaseURL = conf.String("baseurl")
		l.pathMaps = conf.StringMap("mappings")
		l.SSL = conf.Bool("ssl")
		l.Log.Debugf("Lidarr Path Maps: %v", l.pathMaps)
	} else {
		l.Process = false
	}
}

func (l *Lidarr) MatchPath(path string) bool {
	lidarrFolders, _ := l.server.GetRootFolders()
	for _, folder := range lidarrFolders {
		l.Log.Debug(fmt.Sprintf("checking lidarr %s for %s", folder.Path, path))
		if strings.Contains(l.translatePath(path), folder.Path) {
			return true
		}
	}
	return false
}

func (l *Lidarr) RemoveFile(path string) bool {
	var albumID int64
	var artistID int64
	var trackID int64
	var albumPath string

	artists, _ := l.server.GetArtist("")
	for _, artist := range artists {
		if strings.Contains(l.translatePath(path), artist.Path) {
			artistID = artist.ID
		}
	}

	albums, _ := l.server.GetAlbum("")
	for _, album := range albums {
		if strings.Contains(l.translatePath(path), album.Artist.Path) {
			albumID = album.ID
			albumPath = album.Artist.Path
		}
	}

	trackFiles, _ := l.server.GetTrackFilesForAlbum(albumID)
	for _, trackFile := range trackFiles {
		if trackFile.Path == l.translatePath(path) {
			trackID = trackFile.ID
		}
	}

	if trackID != 0 {
		err := l.server.DeleteTrackFile(trackID)
		if err != nil {
			l.Log.Error(fmt.Sprintf("error deleting track file %d: %v", trackID, err.Error()))
			return false
		}
		l.server.SendCommand(&lidarr.CommandRequest{Name: "RescanFolder", Folders: []string{albumPath}})
		l.server.SendCommand(&lidarr.CommandRequest{Name: "RefreshArtist", ArtistID: artistID})

		return true
	}
	return false
}

func (l *Lidarr) Connect() (bool, string) {
	if l.Process {
		if l.ApiKey != "" {
			protocol := "http"
			if l.SSL {
				protocol = "https"
			}
			l.config = starr.New(l.ApiKey, fmt.Sprintf("%s://%s:%v%v", protocol, l.Address, l.Port, l.BaseURL), 0)
			l.server = lidarr.New(l.config)
			status, err := l.server.GetSystemStatus()
			if err != nil {
				return false, err.Error()
			}

			if status.Version != "" {
				message := l.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ArrConnected",
					TemplateData: map[string]interface{}{
						"Service": "Lidarr",
					},
				})
				return true, message
			}
		} else {
			message := l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrMissingArgs",
				TemplateData: map[string]interface{}{
					"Service": "Lidarr",
				},
			})
			return false, message
		}
	}
	message := l.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "ArrNoOp",
		TemplateData: map[string]interface{}{
			"Service": "Lidarr",
		},
	})
	return false, message
}

func (l Lidarr) translatePath(path string) string {
	keys := make([]string, 0, len(l.pathMaps))
	for k := range l.pathMaps {
		keys = append(keys, k)
	}
	for _, key := range keys {
		if strings.Contains(path, l.pathMaps[key]) {
			message := l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapKey",
				TemplateData: map[string]interface{}{
					"Key": key,
				},
			})
			l.Log.Debug(message)
			message = l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapValue",
				TemplateData: map[string]interface{}{
					"Value": l.pathMaps[key],
				},
			})
			l.Log.Debug(message)
			message = l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapOriginal",
				TemplateData: map[string]interface{}{
					"Path": path,
				},
			})
			l.Log.Debug(message)
			replaced := strings.Replace(path, l.pathMaps[key], key, -1)
			message = l.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ArrDebugPathMapOriginal",
				TemplateData: map[string]interface{}{
					"Path": replaced,
				},
			})
			l.Log.Debug(message)
			return replaced
		}
	}
	return path
}
