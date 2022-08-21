/*
Copyright © 2022 Dustin Essington <aetaric@gmail.com>

*/
package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/kalafut/imohash"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
	"golift.io/starr"
	"golift.io/starr/radarr"
	"golift.io/starr/sonarr"
	"gopkg.in/vansante/go-ffprobe.v2"
)

// Sonarr Vars
var sonarrConfig *starr.Config
var sonarrServer *sonarr.Sonarr
var processSonarr bool
var sonarrApiKey string
var sonarrAddress string
var sonarrPort int
var sonarrBaseUrl string

// Radarr Vars
var radarrConfig *starr.Config
var radarrServer *radarr.Radarr
var processRadarr bool
var radarrApiKey string
var radarrAddress string
var radarrPort int
var radarrBaseUrl string

// Command Vars
var checkPath []string
var debug bool
var unknownFiles bool
var dbPath string

var db *bolt.DB

// Stats Vars
var sonarrSubmissions uint64 = 0
var radarrSubmissions uint64 = 0
var filesChecked uint64 = 0
var hashMatches uint64 = 0
var hashMismatches uint64 = 0
var videoFiles uint64 = 0
var unknownFileCount uint64 = 0
var unknownFilesDeleted uint64 = 0
var nonVideo uint64 = 0
var startTime time.Time
var endTime time.Time

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check files in the spcified path for issues",
	Long:  `Runs a loop of all files int he specified path, checking to make sure they are media files`,
	Run: func(cmd *cobra.Command, args []string) {
		startTime = time.Now()
		var err error

		db, err = bolt.Open(dbPath, 0600, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		if processSonarr {
			if sonarrApiKey != "" {
				sonarrConfig = starr.New(sonarrApiKey, fmt.Sprintf("http://%s:%v%v", sonarrAddress, sonarrPort, sonarrBaseUrl), 0)
				sonarrServer = sonarr.New(sonarrConfig)
				status, err := sonarrServer.GetSystemStatus()
				if err != nil {
					panic(err)
				}

				if status.Version != "" {
					log.Println("Sonarr Connected.")
				}
			} else {
				log.Panicln("Missing Sonarr arguments")
			}

		} else {
			log.Println("Sonarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
		}

		if processRadarr {
			if radarrApiKey != "" {
				radarrConfig = starr.New(radarrApiKey, fmt.Sprintf("http://%s:%v%v", radarrAddress, radarrPort, radarrBaseUrl), 0)
				radarrServer = radarr.New(radarrConfig)
				status, err := radarrServer.GetSystemStatus()
				if err != nil {
					panic(err)
				}

				if status.Version != "" {
					log.Println("Radarr Connected.")
				}
			} else {
				log.Panicln("Missing Radarr arguments")
			}

		} else {
			log.Println("Radarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
		}

		if unknownFiles {
			log.Println(`WARNING: unknown file deletion is on. You may lose files that are not tracked by sonarr or radarr. This will still delete files even if you have sonarr and radarr disabled.`)
		}

		db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})

		for _, path := range checkPath {
			filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					log.Fatalf(err.Error()+" %v", path)
					return err
				}

				if !info.IsDir() {
					filesChecked++
					var hash = []byte(nil)

					err := db.View(func(tx *bolt.Tx) error {
						b := tx.Bucket([]byte("Checkrr"))
						v := b.Get([]byte(path))
						if v != nil {
							hash = v
						}
						return nil
					})
					if err != nil {
						log.Fatalf("Error accessing database: %v", err.Error())
					}

					if hash == nil {
						if debug {
							log.Print("DB Hash: not found")
						}
						checkFile(path)
					} else {
						if debug {
							log.Printf("DB Hash: %x", hash)
						}

						filehash := imohash.New()
						sum, _ := filehash.SumFile(path)

						if debug {
							log.Printf("File Hash: %x", sum)
						}

						if hex.EncodeToString(sum[:]) != hex.EncodeToString(hash[:]) {
							log.Printf("Hash mismatch - \"%v\"", path)
							hashMismatches++
							checkFile(path)
						} else {
							log.Printf("Hash match - \"%v\"", path)
							hashMatches++
						}
					}
				}
				return nil
			})
		}
		endTime = time.Now()
		diff := endTime.Sub(startTime)
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendRows([]table.Row{
			{"Files Checked", filesChecked},
			{"Hash Matches", hashMatches},
			{"Hashes Mismatched", hashMismatches},
			{"Submitted to Sonarr", sonarrSubmissions},
			{"Submitted to Radarr", radarrSubmissions},
			{"Video Files", videoFiles},
			{"Non-Video Files", nonVideo},
			{"Unknown Files", unknownFileCount},
			{"Unknown File Deletes", unknownFilesDeleted},
			{"Elapsed Time", diff},
		})
		t.Render()
	},
}

func deleteFile(path string) bool {

	var target string

	if processSonarr {
		sonarrFolders, _ := sonarrServer.GetRootFolders()
		for _, folder := range sonarrFolders {
			if strings.Contains(path, folder.Path) {
				target = "sonarr"
			}
		}
	}
	if processRadarr {
		radarrFolders, _ := radarrServer.GetRootFolders()
		for _, folder := range radarrFolders {
			if strings.Contains(path, folder.Path) {
				target = "radarr"
			}
		}
	}

	if target == "sonarr" && processSonarr {
		var seriesID int64
		seriesList, _ := sonarrServer.GetAllSeries()
		for _, series := range seriesList {
			if strings.Contains(path, series.Path) {
				seriesID = series.ID
				files, _ := sonarrServer.GetSeriesEpisodeFiles(seriesID)
				for _, file := range files {
					if file.Path == path {
						sonarrServer.DeleteEpisodeFile(file.ID)
						sonarrServer.SendCommand(&sonarr.CommandRequest{Name: "RescanSeries", SeriesID: seriesID})
						sonarrServer.SendCommand(&sonarr.CommandRequest{Name: "SeriesSearch", SeriesID: seriesID})
						log.Printf("Submitted \"%v\" to Sonarr to reaquire", path)
						sonarrSubmissions++
						return true
					}
				}
			}
		}
	} else if target == "radarr" && processRadarr {
		var movieID int64
		var movieIDs []int64
		movieList, _ := radarrServer.GetMovie(0)
		for _, movie := range movieList {
			if strings.Contains(path, movie.Path) {
				movieID = movie.ID
				movieIDs = append(movieIDs, movieID)
				ctx, cancelfunc := context.WithTimeout(context.Background(), 300*time.Second)
				defer cancelfunc()
				radarrServer.APIer.Delete(ctx, fmt.Sprintf("/api/v3/moviefile/%v", movie.MovieFile.ID), nil)
				radarrServer.SendCommand(&radarr.CommandRequest{Name: "RefreshMovie", MovieIDs: movieIDs})
				radarrServer.SendCommand(&radarr.CommandRequest{Name: "MoviesSearch", MovieIDs: movieIDs})
				log.Printf("Submitted \"%v\" to Radarr to reaquire", path)
				radarrSubmissions++
				return true
			}
		}
	} else {
		log.Printf("Couldn't find a target for file \"%v\". File is unknown.", path)
		return unknownDelete(path)
	}
	return false
}

func checkFile(path string) bool {
	ctx := context.Background()

	buf, _ := ioutil.ReadFile(path)
	if filetype.IsVideo(buf) {
		videoFiles++
		data, err := ffprobe.ProbeURL(ctx, path)
		if err != nil {
			log.Printf("Error getting data: %v - %v", err, path)
			return deleteFile(path)
		} else {
			log.Println(string(data.Format.FormatLongName) + " - " + string(data.Format.Filename))

			filehash := imohash.New()
			sum, _ := filehash.SumFile(path)

			if debug {
				log.Printf("New File Hash: %x", sum)
			}
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("Checkrr"))
				err := b.Put([]byte(path), sum[:])
				return err
			})
			if err != nil {
				log.Printf("Error: %v", err.Error())
			}
			return true
		}
	} else if filetype.IsAudio(buf) || filetype.IsImage(buf) || filetype.IsDocument(buf) || http.DetectContentType(buf) == "text/plain; charset=utf-8" {
		log.Printf("File \"%v\" is an image, audio, or subtitle file, skipping...", path)
		nonVideo++
		return true
	} else {
		content := http.DetectContentType(buf)
		if debug {
			log.Printf("File \"%v\" is of type \"%v\"", path, content)
		}
		log.Printf("File \"%v\" is not a recongized file type", path)
		unknownFileCount++
		return deleteFile(path)
	}
}

func unknownDelete(path string) bool {
	if unknownFiles {
		e := os.Remove(path)
		if e != nil {
			log.Printf("Could not delete File: \"%v\"", path)
			return false
		}
		log.Printf("Removed File: \"%v\"", path)
		unknownFilesDeleted++
		return true
	}
	return false
}

func init() {
	// Here you will define your flags and configuration settings.
	checkCmd.PersistentFlags().StringVar(&sonarrApiKey, "sonarrApiKey", "", "API Key for Sonarr")
	viper.GetViper().BindPFlag("sonarrapikey", checkCmd.PersistentFlags().Lookup("sonarrApiKey"))
	checkCmd.PersistentFlags().StringVar(&sonarrAddress, "sonarrAddress", "127.0.0.1", "Address for Sonarr")
	viper.GetViper().BindPFlag("sonarraddress", checkCmd.PersistentFlags().Lookup("sonarrAddress"))
	checkCmd.PersistentFlags().IntVar(&sonarrPort, "sonarrPort", 8989, "Port for Sonarr")
	viper.GetViper().BindPFlag("sonarrport", checkCmd.PersistentFlags().Lookup("sonarrPort"))
	checkCmd.PersistentFlags().StringVar(&sonarrBaseUrl, "sonarrBaseUrl", "/", "Base URL for Sonarr")
	viper.GetViper().BindPFlag("sonarrbaseurl", checkCmd.PersistentFlags().Lookup("sonarrBaseUrl"))

	checkCmd.PersistentFlags().BoolVar(&processSonarr, "processSonarr", false, "Delete files via Sonarr, rescan the series, and search for replacements")
	viper.GetViper().BindPFlag("processsonarr", checkCmd.PersistentFlags().Lookup("processSonarr"))

	checkCmd.PersistentFlags().StringVar(&radarrApiKey, "radarrApiKey", "", "API Key for Radarr")
	viper.GetViper().BindPFlag("radarrapikey", checkCmd.PersistentFlags().Lookup("radarrApiKey"))
	checkCmd.PersistentFlags().StringVar(&radarrAddress, "radarrAddress", "", "Address for Radarr")
	viper.GetViper().BindPFlag("radarraddress", checkCmd.PersistentFlags().Lookup("radarrAddress"))
	checkCmd.PersistentFlags().IntVar(&radarrPort, "radarrPort", 7878, "Port for Radarr")
	viper.GetViper().BindPFlag("radarrport", checkCmd.PersistentFlags().Lookup("radarrPort"))
	checkCmd.PersistentFlags().StringVar(&radarrBaseUrl, "radarrBaseUrl", "/", "Base URL for Radarr")
	viper.GetViper().BindPFlag("radarrbaseurl", checkCmd.PersistentFlags().Lookup("radarrBaseUrl"))

	checkCmd.PersistentFlags().BoolVar(&processRadarr, "processRadarr", false, "Delete files via Radarr, rescan the movie, and search for replacements")
	viper.GetViper().BindPFlag("processradarr", checkCmd.PersistentFlags().Lookup("processRadarr"))

	checkCmd.PersistentFlags().StringArrayVar(&checkPath, "checkPath", []string{}, "Path(s) to check")
	checkCmd.MarkPersistentFlagRequired("checkPath")
	viper.GetViper().BindPFlag("checkpath", checkCmd.PersistentFlags().Lookup("checkPath"))

	checkCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Turn on Debug Messages")
	checkCmd.PersistentFlags().BoolVar(&unknownFiles, "removeUnknownFiles", false, "Deletes any unknown files from the disk. This is probably a bad idea. Seriously.")
	viper.GetViper().BindPFlag("removeunknownfiles", checkCmd.PersistentFlags().Lookup("removeUnknownFiles"))

	checkCmd.PersistentFlags().StringVar(&dbPath, "database", "checkrr.db", "Path to checkrr database")
	checkCmd.MarkPersistentFlagRequired("database")
	checkCmd.MarkPersistentFlagFilename("database", "db")
	viper.GetViper().BindPFlag("database", checkCmd.PersistentFlags().Lookup("database"))

	rootCmd.AddCommand(checkCmd)

}
