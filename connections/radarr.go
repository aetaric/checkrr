package connections

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/viper"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

type Radarr struct {
	config  *starr.Config
	server  *radarr.Radarr
	Process bool
	ApiKey  string
	Address net.IPAddr
	Port    int
	BaseURL string
}

func (r *Radarr) FromConfig(conf viper.Viper) {
	r.Address = net.IPAddr{IP: net.ParseIP(viper.GetString("address"))}
	r.Process = viper.GetBool("process")
	r.ApiKey = viper.GetString("apikey")
	r.Port = viper.GetInt("port")
	r.BaseURL = viper.GetString("baseurl")
}

func (r *Radarr) MatchPath(path string) bool {
	radarrFolders, _ := r.server.GetRootFolders()
	for _, folder := range radarrFolders {
		if strings.Contains(path, folder.Path) {
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
		if strings.Contains(path, movie.Path) {
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
