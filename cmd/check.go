/*
Copyright © 2022 Dustin Essington <aetaric@gmail.com>

*/
package cmd

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/aetaric/checkrr/hidden"
	"github.com/aetaric/checkrr/notifications"
	"github.com/h2non/filetype"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/kalafut/imohash"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
	"golift.io/starr"
	"golift.io/starr/lidarr"
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

// Lidarr Vars
var lidarrConfig *starr.Config
var lidarrServer *lidarr.Lidarr
var processLidarr bool
var lidarrApiKey string
var lidarrAddress string
var lidarrPort int
var lidarrBaseUrl string

// Command Vars
var checkPath []string
var ignoreExts []string
var ignoreHidden bool = false
var debug bool
var unknownFiles bool
var dbPath string
var logFile string
var csvFile string
var csvFileWriter *csv.Writer
var discordWebhook string
var discordNotification notifications.DiscordWebhook
var notificationTypes []string
var logJSON bool = false

var db *bolt.DB

// Stats Vars
var sonarrSubmissions uint64 = 0
var radarrSubmissions uint64 = 0
var lidarrSubmissions uint64 = 0
var filesChecked uint64 = 0
var hashMatches uint64 = 0
var hashMismatches uint64 = 0
var videoFiles uint64 = 0
var audioFiles uint64 = 0
var unknownFileCount uint64 = 0
var unknownFilesDeleted uint64 = 0
var nonVideo uint64 = 0
var startTime time.Time
var endTime time.Time

// pprof
var profileCode bool = false

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check files in the spcified path for issues",
	Long:  `Runs a loop of all files in the specified path(s), checking to make sure they are media files`,
	Run: func(cmd *cobra.Command, args []string) {

		if debug {
			log.SetLevel(log.DebugLevel)
		}

		if logJSON {
			log.SetFormatter(&log.JSONFormatter{})
		}

		log.SetOutput(os.Stdout)

		_, binpatherr := exec.LookPath("ffprobe")
		if binpatherr != nil {
			log.WithFields(log.Fields{"startup": true}).Fatal("Failed to find ffprobe in your path... Please install FFProbe (typically included with the FFMPEG package) and make sure it is in your $PATH var. Exiting...")
		}

		if profileCode {
			f, err := os.Create(".checkrr.prof")
			if err != nil {
				log.WithFields(log.Fields{"startup": true}).Fatal(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
			defer f.Close()
		}

		if logFile != "" {
			f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.WithFields(log.Fields{"startup": true}).Fatalf("error opening log file: %v", err)
			}
			defer f.Close()

			log.SetOutput(f)
		}

		if csvFile != "" {
			csvFileHandle, err := os.Create(csvFile)
			if err != nil {
				log.WithFields(log.Fields{"startup": true}).Fatalf("failed creating file: %s", err)
			}
			defer csvFileHandle.Close()
			csvFileWriter = csv.NewWriter(csvFileHandle)
			defer csvFileWriter.Flush()
		}

		// Fixes a bug with viper not reading in the string slice from the config while still allowing commandline additions
		if checkPath[0] == "" {
			checkPath = viper.GetViper().GetStringSlice("checkpath")
		}

		startTime = time.Now()

		var err error

		db, err = bolt.Open(dbPath, 0600, nil)
		if err != nil {
			log.WithFields(log.Fields{"startup": true}).Fatal(err)
		}
		defer db.Close()

		if processSonarr {
			if sonarrApiKey != "" {
				sonarrConfig = starr.New(sonarrApiKey, fmt.Sprintf("http://%s:%v%v", sonarrAddress, sonarrPort, sonarrBaseUrl), 0)
				sonarrServer = sonarr.New(sonarrConfig)
				status, err := sonarrServer.GetSystemStatus()
				if err != nil {
					log.Fatal(err)
				}

				if status.Version != "" {
					log.WithFields(log.Fields{"startup": true}).Info("Sonarr Connected.")
				}
			} else {
				log.WithFields(log.Fields{"startup": true}).Fatal("Missing Sonarr arguments")
			}

		} else {
			log.WithFields(log.Fields{"startup": true}).Warn("Sonarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
		}

		if processRadarr {
			if radarrApiKey != "" {
				radarrConfig = starr.New(radarrApiKey, fmt.Sprintf("http://%s:%v%v", radarrAddress, radarrPort, radarrBaseUrl), 0)
				radarrServer = radarr.New(radarrConfig)
				status, err := radarrServer.GetSystemStatus()
				if err != nil {
					log.Fatal(err)
				}

				if status.Version != "" {
					log.WithFields(log.Fields{"startup": true}).Info("Radarr Connected.")
				}
			} else {
				log.WithFields(log.Fields{"startup": true}).Fatal("Missing Radarr arguments")
			}

		} else {
			log.WithFields(log.Fields{"startup": true}).Warn("Radarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
		}

		if processLidarr {
			if lidarrApiKey != "" {
				lidarrConfig = starr.New(lidarrApiKey, fmt.Sprintf("http://%s:%v%v", lidarrAddress, lidarrPort, lidarrBaseUrl), 0)
				lidarrServer = lidarr.New(lidarrConfig)
				status, err := lidarrServer.GetSystemStatus()
				if err != nil {
					log.Fatal(err)
				}

				if status.Version != "" {
					log.WithFields(log.Fields{"startup": true}).Info("Lidarr Connected.")
				}
			} else {
				log.WithFields(log.Fields{"startup": true}).Fatal("Missing Lidarr arguments")
			}

		} else {
			log.WithFields(log.Fields{"startup": true}).Warn("Lidarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
		}

		if unknownFiles {
			log.WithFields(log.Fields{"startup": true, "unknownFiles": "enabled"}).Warn(`unknown file deletion is on. You may lose files that are not tracked by services you've enabled in the config. This will still delete files even if those integrations are disabled.`)
		}

		if discordWebhook != "" {
			discordNotification := notifications.DiscordWebhook{URL: discordWebhook, AllowedNotifs: notificationTypes, Log: log.Logger{}}

			if notifications.DiscordWebhook.Connect(discordNotification) {
				log.WithFields(log.Fields{"startup": true}).Info("Connected to Discord")
				discordNotification.Connected = true
			} else {
				log.WithFields(log.Fields{"startup": true}).Warn("Error connecting to Discord")
			}
		}

		db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})

		for _, path := range checkPath {
			log.WithFields(log.Fields{"startup": true}).Debug("Path: %v", path)

			filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					log.Fatalf(err.Error()+" %v", path)
					return err
				}

				if !d.IsDir() {
					var ignore bool = false

					ext := filepath.Ext(path)
					for _, v := range ignoreExts {
						if v == ext {
							ignore = true
						}
					}

					if ignoreHidden {
						i, _ := hidden.IsHidden(path)
						ignore = i
					}

					if !ignore {
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
							log.WithFields(log.Fields{"DB Hash": "Not Found"}).Debugf("DB Hash not found, checking file \"%s\"", path)
							checkFile(path)
						} else {
							log.WithFields(log.Fields{"DB Hash": "Found"}).Debugf("DB Hash: %x", hash)

							filehash := imohash.New()
							sum, _ := filehash.SumFile(path)

							log.WithFields(log.Fields{"DB Hash": "Found", "File Hash": "Computed"}).Debug("File Hash: %x", hex.EncodeToString(sum[:]))

							if hex.EncodeToString(sum[:]) != hex.EncodeToString(hash[:]) {
								log.WithFields(log.Fields{"Hash Match": false}).Infof("\"%v\"", path)
								hashMismatches++
								checkFile(path)
							} else {
								log.WithFields(log.Fields{"Hash Match": true}).Infof("\"%v\"", path)
								hashMatches++
							}
						}
					} else {
						log.WithFields(log.Fields{"Ignored": true}).Debugf("\"%s\"", path)
					}
				}
				return nil
			})
			if profileCode {
				heap, _ := os.Create(".checkrr.heap")
				defer heap.Close()
				pprof.WriteHeapProfile(heap)
			}
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
			{"Submitted to Lidarr", lidarrSubmissions},
			{"Video Files", videoFiles},
			{"Audio Files", audioFiles},
			{"Text or Other Files", nonVideo},
			{"Unknown Files", unknownFileCount},
			{"Unknown File Deletes", unknownFilesDeleted},
			{"Elapsed Time", diff},
		})
		t.Render()
	},
}

func deleteFile(path string) bool {
	var target string
	var deleted bool = false

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
	if processLidarr {
		lidarrFolders, _ := lidarrServer.GetRootFolders()
		for _, folder := range lidarrFolders {
			if strings.Contains(path, folder.Path) {
				target = "lidarr"
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
						log.WithFields(log.Fields{"Submitted to Sonarr": true}).Infof("Submitted \"%v\" to Sonarr to reacquire", path)
						discordNotification.Notify("File sent to Sonarr", fmt.Sprintf("Sent \"%v\" to Sonarr to reaquire.", path), "reaquire")
						deleted = true
						sonarrSubmissions++
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
				edit := radarr.BulkEdit{MovieIDs: []int64{movie.MovieFile.MovieID}, DeleteFiles: starr.True()}
				radarrServer.EditMovies(&edit)
				radarrServer.SendCommand(&radarr.CommandRequest{Name: "RefreshMovie", MovieIDs: movieIDs})
				radarrServer.SendCommand(&radarr.CommandRequest{Name: "MoviesSearch", MovieIDs: movieIDs})
				log.WithFields(log.Fields{"Submitted to Radarr": true}).Infof("Submitted \"%v\" to Radarr to reacquire", path)
				discordNotification.Notify("File sent to Radarr", fmt.Sprintf("Sent \"%v\" to Radarr to reaquire.", path), "reacquire")
				deleted = true
				radarrSubmissions++
			}
		}
	} else if target == "lidarr" && processLidarr {

		var albumID int64
		var artistID int64
		var trackID int64
		var albumPath string

		artists, _ := lidarrServer.GetArtist("")
		for _, artist := range artists {
			if strings.Contains(path, artist.Path) {
				artistID = artist.ID
			}
		}

		albums, _ := lidarrServer.GetAlbum("")
		for _, album := range albums {
			if strings.Contains(path, album.Artist.Path) {
				albumID = album.ID
				albumPath = album.Artist.Path
			}
		}

		trackFiles, _ := lidarrServer.GetTrackFilesForAlbum(albumID)
		for _, trackFile := range trackFiles {
			if trackFile.Path == path {
				trackID = trackFile.ID
			}
		}

		if trackID != 0 {
			lidarrServer.DeleteTrackFile(trackID)

			lidarrServer.SendCommand(&lidarr.CommandRequest{Name: "RescanFolder", Folders: []string{albumPath}})
			lidarrServer.SendCommand(&lidarr.CommandRequest{Name: "RefreshArtist", ArtistID: artistID})

			log.WithFields(log.Fields{"Submitted to Lidarr": true}).Infof("Submitted \"%v\" to Lidarr to reacquire", path)
			discordNotification.Notify("File sent to Lidarr", fmt.Sprintf("Sent \"%v\" to Lidarr to reaquire.", path), "reacquire")
			deleted = true
			lidarrSubmissions++
		}
	}

	if !deleted {
		log.WithFields(log.Fields{"Unknown File": true}).Infof("Couldn't find a target for file \"%v\". File is unknown.", path)
		unknownDelete(path)
	}

	if csvFile != "" {
		if deleted {
			csvFileWriter.Write([]string{path, target})
		} else {
			csvFileWriter.Write([]string{path, "unknown"})
		}
	}
	return false
}

func checkFile(path string) bool {
	ctx := context.Background()

	// This seems like an insane number, but it's only 33KB and will allow detection of all file types via the filetype library
	f, _ := os.Open(path)
	defer f.Close()

	buf := make([]byte, 33000)
	f.Read(buf)
	var detectedFileType string

	if filetype.IsVideo(buf) || filetype.IsAudio(buf) {
		if filetype.IsAudio(buf) {
			audioFiles++
			detectedFileType = "Audio"
		} else {
			videoFiles++
			detectedFileType = "Video"
		}
		data, err := ffprobe.ProbeURL(ctx, path)
		if err != nil {
			log.WithFields(log.Fields{"FFProbe": "failed", "Type": detectedFileType}).Warnf("Error getting data: %v - %v", err, path)
			data, buf, err = nil, nil, nil
			return deleteFile(path)
		} else {
			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true}).Infof(string(data.Format.Filename))

			filehash := imohash.New()
			sum, _ := filehash.SumFile(path)

			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "File Hashed": true}).Debugf("New File Hash: %x", sum)

			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("Checkrr"))
				err := b.Put([]byte(path), sum[:])
				return err
			})
			if err != nil {
				log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
			}

			buf, data = nil, nil
			return true
		}
	} else if filetype.IsImage(buf) || filetype.IsDocument(buf) || http.DetectContentType(buf) == "text/plain; charset=utf-8" {
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Other"}).Infof("File \"%v\" is an image or subtitle file, skipping...", path)
		buf = nil
		nonVideo++
		return true
	} else {
		content := http.DetectContentType(buf)
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Debugf("File \"%v\" is of type \"%v\"", path, content)
		buf = nil
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Infof("File \"%v\" is not a recongized file type", path)
		discordNotification.Notify("Unknown file detected", fmt.Sprintf("\"%v\" is not a Video, Audio, Image, Subtitle, or Plaintext file.", path), "unknowndetected")
		unknownFileCount++
		return deleteFile(path)
	}
}

func unknownDelete(path string) bool {
	if unknownFiles {
		e := os.Remove(path)
		if e != nil {
			log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown", "Deleted": false}).Warnf("Could not delete File: \"%v\"", path)
			return false
		}
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown", "Deleted": true}).Warnf("Removed File: \"%v\"", path)
		discordNotification.Notify("Unknown file deleted", fmt.Sprintf("\"%v\" was removed.", path), "unknowndeleted")
		unknownFilesDeleted++
		return true
	}
	return false
}

func init() {
	checkCmd.Flags().StringVar(&sonarrApiKey, "sonarrApiKey", "", "API Key for Sonarr")
	viper.GetViper().BindPFlag("sonarrapikey", checkCmd.Flags().Lookup("sonarrApiKey"))
	checkCmd.Flags().StringVar(&sonarrAddress, "sonarrAddress", "127.0.0.1", "Address for Sonarr")
	viper.GetViper().BindPFlag("sonarraddress", checkCmd.Flags().Lookup("sonarrAddress"))
	checkCmd.Flags().IntVar(&sonarrPort, "sonarrPort", 8989, "Port for Sonarr")
	viper.GetViper().BindPFlag("sonarrport", checkCmd.Flags().Lookup("sonarrPort"))
	checkCmd.Flags().StringVar(&sonarrBaseUrl, "sonarrBaseUrl", "/", "Base URL for Sonarr")
	viper.GetViper().BindPFlag("sonarrbaseurl", checkCmd.Flags().Lookup("sonarrBaseUrl"))

	checkCmd.Flags().BoolVar(&processSonarr, "processSonarr", false, "Delete files via Sonarr, rescan the series, and search for replacements")
	viper.GetViper().BindPFlag("processsonarr", checkCmd.Flags().Lookup("processSonarr"))

	checkCmd.Flags().StringVar(&radarrApiKey, "radarrApiKey", "", "API Key for Radarr")
	viper.GetViper().BindPFlag("radarrapikey", checkCmd.Flags().Lookup("radarrApiKey"))
	checkCmd.Flags().StringVar(&radarrAddress, "radarrAddress", "", "Address for Radarr")
	viper.GetViper().BindPFlag("radarraddress", checkCmd.Flags().Lookup("radarrAddress"))
	checkCmd.Flags().IntVar(&radarrPort, "radarrPort", 7878, "Port for Radarr")
	viper.GetViper().BindPFlag("radarrport", checkCmd.Flags().Lookup("radarrPort"))
	checkCmd.Flags().StringVar(&radarrBaseUrl, "radarrBaseUrl", "/", "Base URL for Radarr")
	viper.GetViper().BindPFlag("radarrbaseurl", checkCmd.Flags().Lookup("radarrBaseUrl"))

	checkCmd.Flags().BoolVar(&processRadarr, "processRadarr", false, "Delete files via Radarr, rescan the movie, and search for replacements")
	viper.GetViper().BindPFlag("processradarr", checkCmd.Flags().Lookup("processRadarr"))

	checkCmd.Flags().StringVar(&lidarrApiKey, "lidarrApiKey", "", "API Key for Lidarr")
	viper.GetViper().BindPFlag("lidarrapikey", checkCmd.Flags().Lookup("lidarrApiKey"))
	checkCmd.Flags().StringVar(&lidarrAddress, "lidarrAddress", "", "Address for Lidarr")
	viper.GetViper().BindPFlag("lidarraddress", checkCmd.Flags().Lookup("lidarrAddress"))
	checkCmd.Flags().IntVar(&lidarrPort, "lidarrPort", 8686, "Port for Lidarr")
	viper.GetViper().BindPFlag("lidarrport", checkCmd.Flags().Lookup("lidarrPort"))
	checkCmd.Flags().StringVar(&lidarrBaseUrl, "lidarrBaseUrl", "/", "Base URL for Lidarr")
	viper.GetViper().BindPFlag("lidarrbaseurl", checkCmd.Flags().Lookup("lidarrBaseUrl"))

	checkCmd.Flags().BoolVar(&processLidarr, "processLidarr", false, "Delete files via Lidarr, rescan the album, and search for replacements")
	viper.GetViper().BindPFlag("processlidarr", checkCmd.Flags().Lookup("processLidarr"))

	checkCmd.PersistentFlags().StringArrayVar(&checkPath, "checkPath", []string{}, "Path(s) to check")
	checkCmd.MarkPersistentFlagRequired("checkPath")
	viper.BindPFlag("checkpath", checkCmd.Flags().Lookup("checkPath"))

	checkCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Turn on Debug Messages")
	checkCmd.Flags().BoolVar(&profileCode, "profileCode", false, "Turn on code profiling")

	checkCmd.Flags().BoolVar(&unknownFiles, "removeUnknownFiles", false, "Deletes any unknown files from the disk. This is probably a bad idea. Seriously.")
	viper.GetViper().BindPFlag("removeunknownfiles", checkCmd.Flags().Lookup("removeUnknownFiles"))

	checkCmd.PersistentFlags().StringVar(&dbPath, "database", "checkrr.db", "Path to checkrr database")
	checkCmd.MarkPersistentFlagRequired("database")
	checkCmd.MarkPersistentFlagFilename("database", "db")
	viper.GetViper().BindPFlag("database", checkCmd.Flags().Lookup("database"))

	checkCmd.PersistentFlags().StringVar(&logFile, "logFile", "", "Path to log file.")
	viper.GetViper().BindPFlag("logfile", checkCmd.Flags().Lookup("logFile"))

	checkCmd.PersistentFlags().StringVar(&csvFile, "csvFile", "", "Output broken files to a CSV file")
	viper.GetViper().BindPFlag("csvfile", checkCmd.Flags().Lookup("csvFile"))

	checkCmd.PersistentFlags().StringVar(&discordWebhook, "discordWebhook", "", "Discord Webhook URL to send notifications to.")
	viper.GetViper().BindPFlag("discordwebhook", checkCmd.Flags().Lookup("discordWebhook"))

	checkCmd.PersistentFlags().BoolVar(&logJSON, "logJSON", false, "Switches the logger to JSON. Default is Plain Text.")
	viper.GetViper().BindPFlag("logjson", checkCmd.Flags().Lookup("logJSON"))

	checkCmd.PersistentFlags().StringArrayVar(&ignoreExts, "ignoreExt", []string{}, "Ignore a file extension")
	viper.BindPFlag("ignoreexts", checkCmd.Flags().Lookup("IgnoreExt"))

	checkCmd.PersistentFlags().BoolVar(&ignoreHidden, "ignoreHidden", false, "Ignores hidden files.")
	viper.GetViper().BindPFlag("ignorehidden", checkCmd.Flags().Lookup("ignoreHidden"))

	checkCmd.PersistentFlags().StringSliceVar(&notificationTypes, "notificationTypes", []string{}, "List of Notification Types to send notifications for.")
	viper.GetViper().BindPFlag("notificationTypes", checkCmd.Flags().Lookup("notificationTypes"))

	rootCmd.AddCommand(checkCmd)
}
