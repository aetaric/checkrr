package check

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aetaric/checkrr/connections"
	"github.com/aetaric/checkrr/features"
	"github.com/aetaric/checkrr/hidden"
	"github.com/aetaric/checkrr/notifications"
	"github.com/h2non/filetype"
	"github.com/kalafut/imohash"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	bolt "go.etcd.io/bbolt"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type Checkrr struct {
	Stats         features.Stats
	DB            *bolt.DB
	Running       bool
	csv           features.CSV
	notifications notifications.Notifications
	sonarr        connections.Sonarr
	radarr        connections.Radarr
	lidarr        connections.Lidarr
	ignoreExts    []string
	ignoreHidden  bool
	config        *viper.Viper
	FullConfig    *viper.Viper
	Chan          *chan []string
}

func (c *Checkrr) Run() {

	// Prevent multiple checkrr goroutines from running
	if !c.Running {
		log.Debug("Setting Lock to prevent multi-runs")
		c.Running = true
	} else {
		log.Error("Tried to run more than one check at a time. Adjust your cron timing. If this is your first run, use --run-once.")
		return
	}

	c.Stats = features.Stats{Log: *log.StandardLogger(), DB: c.DB}

	if c.FullConfig.Sub("stats") != nil {
		c.Stats.FromConfig(*c.FullConfig.Sub("stats"))
	}

	// Connect to Sonarr, Radarr, and Lidarr
	c.connectServices()

	// Connect to notifications
	c.connectNotifications()

	// Setup CSV writer
	if c.config.GetString("csvfile") != "" {
		c.csv = features.CSV{FilePath: c.config.GetString("csvfile")}
		c.csv.Open()
	}

	c.ignoreExts = c.config.GetStringSlice("ignoreexts")
	c.ignoreHidden = c.config.GetBool("ignorehidden")

	c.Stats.Start()

	log.Debug(c.config.GetStringSlice("checkpath"))

	for _, path := range c.config.GetStringSlice("checkpath") {
		log.WithFields(log.Fields{"startup": true}).Debug("Path: %v", path)

		filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				log.Fatalf(err.Error()+" %v", path)
				return err
			}
			if !d.IsDir() {
				var ignore bool = false

				ext := filepath.Ext(path)
				for _, v := range c.ignoreExts {
					if v == ext {
						ignore = true
					}
				}

				if c.ignoreHidden {
					i, _ := hidden.IsHidden(path)
					ignore = i
				}

				if !ignore {
					c.Stats.FilesChecked++
					c.Stats.Write("FilesChecked", c.Stats.FilesChecked)
					var hash = []byte(nil)

					err := c.DB.View(func(tx *bolt.Tx) error {
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
						c.checkFile(path)
					} else {
						log.WithFields(log.Fields{"DB Hash": "Found"}).Debugf("DB Hash: %x", hash)

						filehash := imohash.New()
						sum, _ := filehash.SumFile(path)

						log.WithFields(log.Fields{"DB Hash": "Found", "File Hash": "Computed"}).Debug("File Hash: %x", hex.EncodeToString(sum[:]))

						if hex.EncodeToString(sum[:]) != hex.EncodeToString(hash[:]) {
							log.WithFields(log.Fields{"Hash Match": false}).Infof("\"%v\"", path)
							c.Stats.HashMismatches++
							c.Stats.Write("HashMismatches", c.Stats.HashMismatches)
							c.checkFile(path)
						} else {
							log.WithFields(log.Fields{"Hash Match": true}).Infof("\"%v\"", path)
							c.Stats.HashMatches++
							c.Stats.Write("HashMatches", c.Stats.HashMatches)
						}
					}
				} else {
					log.WithFields(log.Fields{"Ignored": true}).Debugf("\"%s\"", path)
				}
			}
			return nil
		})
	}

	c.notifications.Notify("Checkrr Finished", "A checkrr run has ended", "endrun", "")
	c.Stats.Stop()
	c.Stats.Render()
	c.Running = false
	ch := *c.Chan
	ch <- []string{"time"}
}

func (c *Checkrr) FromConfig(conf *viper.Viper) {
	c.config = conf
}

func (c *Checkrr) connectServices() {
	if viper.GetViper().Sub("sonarr") != nil {
		c.sonarr = connections.Sonarr{}
		c.sonarr.FromConfig(*viper.GetViper().Sub("sonarr"))
		sonarrConnected, sonarrMessage := c.sonarr.Connect()
		log.WithFields(log.Fields{"Startup": true, "Sonarr Connected": sonarrConnected}).Info(sonarrMessage)
	} else {
		log.WithFields(log.Fields{"Startup": true, "Sonarr Connected": false}).Info("Sonarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
	}

	if viper.GetViper().Sub("radarr") != nil {
		c.radarr = connections.Radarr{}
		c.radarr.FromConfig(*viper.GetViper().Sub("radarr"))
		radarrConnected, radarrMessage := c.radarr.Connect()
		log.WithFields(log.Fields{"Startup": true, "Radarr Connected": radarrConnected}).Info(radarrMessage)
	} else {
		log.WithFields(log.Fields{"Startup": true, "Radarr Connected": false}).Info("Radarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
	}

	if viper.GetViper().Sub("lidarr") != nil {
		c.lidarr = connections.Lidarr{}
		c.lidarr.FromConfig(*viper.GetViper().Sub("lidarr"))
		lidarrConnected, lidarrMessage := c.lidarr.Connect()
		log.WithFields(log.Fields{"Startup": true, "Lidarr Connected": lidarrConnected}).Info(lidarrMessage)
	} else {
		log.WithFields(log.Fields{"Startup": true, "Lidarr Connected": false}).Info("Lidarr integration not enabled. Files will not be fixed. (if you expected a no-op, this is fine)")
	}
}

func (c *Checkrr) connectNotifications() {
	if viper.GetViper().Sub("notifications") != nil {
		c.notifications = notifications.Notifications{Log: *log.StandardLogger()}
		c.notifications.FromConfig(*viper.GetViper().Sub("notifications"))
		c.notifications.Connect()
	} else {
		log.WithFields(log.Fields{"Startup": true, "Notifications Connected": false}).Warn("No config options for notifications found.")
	}
	c.notifications.Notify("Checkrr Starting", "A checkrr run has begun", "startrun", "")
}

func (c *Checkrr) checkFile(path string) {
	ctx := context.Background()

	// This seems like an insane number, but it's only 33KB and will allow detection of all file types via the filetype library
	f, _ := os.Open(path)
	defer f.Close()

	buf := make([]byte, 33000)
	f.Read(buf)
	var detectedFileType string

	if filetype.IsVideo(buf) || filetype.IsAudio(buf) {
		if filetype.IsAudio(buf) {
			c.Stats.AudioFiles++
			c.Stats.Write("AudioFiles", c.Stats.AudioFiles)
			detectedFileType = "Audio"
		} else {
			c.Stats.VideoFiles++
			c.Stats.Write("VideoFiles", c.Stats.VideoFiles)
			detectedFileType = "Video"
		}
		data, err := ffprobe.ProbeURL(ctx, path)
		if err != nil {
			log.WithFields(log.Fields{"FFProbe": "failed", "Type": detectedFileType}).Warnf("Error getting data: %v - %v", err, path)
			c.deleteFile(path)
			data, buf, err = nil, nil, nil
			return
		} else {
			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true}).Infof(string(data.Format.Filename))

			filehash := imohash.New()
			sum, _ := filehash.SumFile(path)

			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "File Hashed": true}).Debugf("New File Hash: %x", sum)

			err := c.DB.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("Checkrr"))
				err := b.Put([]byte(path), sum[:])
				return err
			})
			if err != nil {
				log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
			}

			buf, data = nil, nil
			return
		}
	} else if filetype.IsImage(buf) || filetype.IsDocument(buf) || http.DetectContentType(buf) == "text/plain; charset=utf-8" {
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Other"}).Infof("File \"%v\" is an image or subtitle file, skipping...", path)
		buf = nil
		c.Stats.NonVideo++
		c.Stats.Write("NonVideo", c.Stats.NonVideo)
		return
	} else {
		content := http.DetectContentType(buf)
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Debugf("File \"%v\" is of type \"%v\"", path, content)
		buf = nil
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Infof("File \"%v\" is not a recongized file type", path)
		c.notifications.Notify("Unknown file detected", fmt.Sprintf("\"%v\" is not a Video, Audio, Image, Subtitle, or Plaintext file.", path), "unknowndetected", path)
		c.Stats.UnknownFileCount++
		c.Stats.Write("UnknownFiles", c.Stats.UnknownFileCount)
		c.deleteFile(path)
		return
	}
}

func (c *Checkrr) deleteFile(path string) {
	if c.sonarr.Process && c.sonarr.MatchPath(path) {
		c.sonarr.RemoveFile(path)
		c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to sonarr to be reacquired", path), "reacquire", path)
		c.Stats.SonarrSubmissions++
		c.Stats.Write("Sonarr", c.Stats.SonarrSubmissions)
		c.recordBadFile(path, "sonarr")
	} else if c.radarr.Process && c.radarr.MatchPath(path) {
		c.radarr.RemoveFile(path)
		c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to radarr to be reacquired", path), "reacquire", path)
		c.Stats.RadarrSubmissions++
		c.Stats.Write("Radarr", c.Stats.RadarrSubmissions)
		c.recordBadFile(path, "radarr")
	} else if c.lidarr.Process && c.lidarr.MatchPath(path) {
		c.lidarr.RemoveFile(path)
		c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to lidarr to be reacquired", path), "reacquire", path)
		c.Stats.LidarrSubmissions++
		c.Stats.Write("Lidarr", c.Stats.LidarrSubmissions)
		c.recordBadFile(path, "lidarr")
	} else {
		log.WithFields(log.Fields{"Unknown File": true}).Infof("Couldn't find a target for file \"%v\". File is unknown.", path)
		c.recordBadFile(path, "unknown")
	}
}

func (c *Checkrr) recordBadFile(path string, fileType string) {

	bad := BadFile{}
	if fileType != "unknown" {
		bad.Reacquire = true
	} else {
		bad.Reacquire = false
	}

	bad.Service = fileType
	bad.FileExt = filepath.Ext(path)

	err := c.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-files"))
		j, e := json.Marshal(bad)
		if e == nil {
			err := b.Put([]byte(path), j)
			return err
		} else {
			return nil
		}
	})
	if err != nil {
		log.WithFields(log.Fields{"DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}
}

type BadFile struct {
	FileExt   string `json:"fileExt"`
	Reacquire bool   `json:"reacquire"`
	Service   string `json:"service"`
}
