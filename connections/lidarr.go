package connections

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/viper"
	"golift.io/starr"
	"golift.io/starr/lidarr"
)

type Lidarr struct {
	config  *starr.Config
	server  *lidarr.Lidarr
	Process bool
	ApiKey  string
	Address net.IPAddr
	Port    int
	BaseURL string
}

func (l *Lidarr) FromConfig(conf viper.Viper) {
	l.Address = net.IPAddr{IP: net.ParseIP(viper.GetString("address"))}
	l.Process = viper.GetBool("process")
	l.ApiKey = viper.GetString("apikey")
	l.Port = viper.GetInt("port")
	l.BaseURL = viper.GetString("baseurl")
}

func (l *Lidarr) MatchPath(path string) bool {
	lidarrFolders, _ := l.server.GetRootFolders()
	for _, folder := range lidarrFolders {
		if strings.Contains(path, folder.Path) {
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
		if strings.Contains(path, artist.Path) {
			artistID = artist.ID
		}
	}

	albums, _ := l.server.GetAlbum("")
	for _, album := range albums {
		if strings.Contains(path, album.Artist.Path) {
			albumID = album.ID
			albumPath = album.Artist.Path
		}
	}

	trackFiles, _ := l.server.GetTrackFilesForAlbum(albumID)
	for _, trackFile := range trackFiles {
		if trackFile.Path == path {
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
