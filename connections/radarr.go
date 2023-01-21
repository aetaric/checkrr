package connections

import (
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

type Radarr struct {
	config   *starr.Config
	server   *radarr.Radarr
	Process  bool
	ApiKey   string
	Address  net.IPAddr
	Port     int
	BaseURL  string
	pathMaps map[string]string
}

func (r *Radarr) FromConfig(conf *viper.Viper) {
	if conf != nil {
		r.Address = net.IPAddr{IP: net.ParseIP(conf.GetString("address"))}
		r.Process = conf.GetBool("process")
		r.ApiKey = conf.GetString("apikey")
		r.Port = conf.GetInt("port")
		r.BaseURL = conf.GetString("baseurl")
		r.pathMaps = conf.GetStringMapString("mappings")
		log.Debug("Path maps: %v", r.pathMaps)
	} else {
		r.Process = false
	}
}

func (r *Radarr) MatchPath(path string) bool {
	radarrFolders, _ := r.server.GetRootFolders()
	for _, folder := range radarrFolders {
		if strings.Contains(r.translatePath(path), folder.Path) {
			return true
		}
	}
	return false
}

func (r *Radarr) RemoveFile(path string) bool {
	var movieID int64
	var movieIDs []int64
	movieList, _ := r.server.GetMovie(0)
	for _, movie := range movieList {
		if strings.Contains(r.translatePath(path), movie.Path) {
			movieID = movie.ID
			movieIDs = append(movieIDs, movieID)
			edit := radarr.BulkEdit{MovieIDs: []int64{movie.MovieFile.MovieID}, DeleteFiles: starr.True()}
			r.server.EditMovies(&edit)
			r.server.SendCommand(&radarr.CommandRequest{Name: "RefreshMovie", MovieIDs: movieIDs})
			r.server.SendCommand(&radarr.CommandRequest{Name: "MoviesSearch", MovieIDs: movieIDs})
			return true
		}
	}
	return false
}

func (r *Radarr) Connect() (bool, string) {
	if r.Process {
		if r.ApiKey != "" {
			r.config = starr.New(r.ApiKey, fmt.Sprintf("http://%s:%v%v", r.Address.IP.String(), r.Port, r.BaseURL), 0)
			r.server = radarr.New(r.config)
			status, err := r.server.GetSystemStatus()
			if err != nil {
				return false, err.Error()
			}

			if status.Version != "" {
				return true, "Radarr Connected."
			}
		} else {
			return false, "Missing Radarr arguments"
		}
	}
	return false, "Radarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)"
}

func (r Radarr) translatePath(path string) string {
	keys := make([]string, 0, len(r.pathMaps))
	for k := range r.pathMaps {
		keys = append(keys, k)
	}
	for _, key := range keys {
		if strings.Contains(path, key) {
			log.Debugf("Replaced path: %s", strings.Replace(path, key, r.pathMaps[key], 1))
			return strings.Replace(path, key, r.pathMaps[key], 1)
		}
	}
	return path
}
