package connections

import (
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golift.io/starr"
	"golift.io/starr/lidarr"
)

type Lidarr struct {
	config   *starr.Config
	server   *lidarr.Lidarr
	Process  bool
	ApiKey   string
	Address  net.IPAddr
	Port     int
	BaseURL  string
	pathMaps map[string]string
}

func (l *Lidarr) FromConfig(conf *viper.Viper) {
	if conf != nil {
		l.Address = net.IPAddr{IP: net.ParseIP(conf.GetString("address"))}
		l.Process = conf.GetBool("process")
		l.ApiKey = conf.GetString("apikey")
		l.Port = conf.GetInt("port")
		l.BaseURL = conf.GetString("baseurl")
		l.pathMaps = conf.GetStringMapString("mappings")
		log.Debug("Lidarr Path Maps: %v", l.pathMaps)
	} else {
		l.Process = false
	}
}

func (l *Lidarr) MatchPath(path string) bool {
	lidarrFolders, _ := l.server.GetRootFolders()
	for _, folder := range lidarrFolders {
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
		l.server.DeleteTrackFile(trackID)

		l.server.SendCommand(&lidarr.CommandRequest{Name: "RescanFolder", Folders: []string{albumPath}})
		l.server.SendCommand(&lidarr.CommandRequest{Name: "RefreshArtist", ArtistID: artistID})

		return true
	}
	return false
}

func (l *Lidarr) Connect() (bool, string) {
	if l.Process {
		if l.ApiKey != "" {
			l.config = starr.New(l.ApiKey, fmt.Sprintf("http://%s:%v%v", l.Address.IP.String(), l.Port, l.BaseURL), 0)
			l.server = lidarr.New(l.config)
			status, err := l.server.GetSystemStatus()
			if err != nil {
				return false, err.Error()
			}

			if status.Version != "" {
				return true, "Lidarr Connected."
			}
		} else {
			return false, "Missing Lidarr arguments"
		}
	}
	return false, "Lidarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)"
}

func (l Lidarr) translatePath(path string) string {
	keys := make([]string, 0, len(l.pathMaps))
	for k := range l.pathMaps {
		keys = append(keys, k)
	}
	for _, key := range keys {
		if strings.Contains(path, l.pathMaps[key]) {
			log.Debugf("Key: %s", key)
			log.Debugf("Value: %s", l.pathMaps[key])
			log.Debugf("Original path: %s", path)
			replaced := strings.Replace(path, l.pathMaps[key], key, -1)
			log.Debugf("New path: %s", replaced)
			return replaced
		}
	}
	return path
}
