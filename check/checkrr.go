package check

import (
	"context"
	"encoding/hex"
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
	Stats        features.Stats
	db           *bolt.DB
	Running      bool
	csv          features.CSV
	discord      notifications.DiscordWebhook
	sonarr       connections.Sonarr
	radarr       connections.Radarr
	lidarr       connections.Lidarr
	ignoreExts   []string
	ignoreHidden bool
	config       *viper.Viper
	Chan         *chan []string
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

	c.Stats = features.Stats{}

	// Connect to Sonarr, Radarr, and Lidarr
	c.connectServices()

	// Unknown File deletion
	if c.config.GetBool("removeunknownfiles") {
		log.WithFields(log.Fields{"startup": true, "unknownFiles": "enabled"}).Warn(`unknown file deletion is on. You may lose files that are not tracked by services you've enabled in the config. This will still delete files even if those integrations are disabled.`)
	}

	// Connect to notifications
	c.connectNotifications()

	// Setup CSV writer
	if c.config.GetString("csvfile") != "" {
		c.csv = features.CSV{FilePath: c.config.GetString("csvfile")}
		c.csv.Open()
	}

	// Setup Database
	if c.config.GetString("database") != "" {
		var err error

		c.db, err = bolt.Open(c.config.GetString("database"), 0600, nil)
		if err != nil {
			log.WithFields(log.Fields{"startup": true}).Fatal(err)
		}
		defer c.db.Close()

		c.db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})
	} else {
		log.WithFields(log.Fields{"startup": true}).Fatal("Database file path missing or unset, please check your config file.")
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
					var hash = []byte(nil)

					err := c.db.View(func(tx *bolt.Tx) error {
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
							c.checkFile(path)
						} else {
							log.WithFields(log.Fields{"Hash Match": true}).Infof("\"%v\"", path)
							c.Stats.HashMatches++
						}
					}
				} else {
					log.WithFields(log.Fields{"Ignored": true}).Debugf("\"%s\"", path)
				}
			}
			return nil
		})
	}

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
	if viper.GetViper().Sub("notifications.discord") != nil {
		c.discord = notifications.DiscordWebhook{}
		c.discord.FromConfig(*viper.GetViper().Sub("notifications.discord"))
		discordConnected, discordMessage := c.discord.Connect()
		log.WithFields(log.Fields{"Startup": true, "Discord Connected": discordConnected}).Info(discordMessage)
	} else {
		log.WithFields(log.Fields{"Startup": true, "Discord Connected": false}).Info("No Discord Webhook URL provided.")
	}
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
			detectedFileType = "Audio"
		} else {
			c.Stats.VideoFiles++
			detectedFileType = "Video"
		}
		data, err := ffprobe.ProbeURL(ctx, path)
		if err != nil {
			log.WithFields(log.Fields{"FFProbe": "failed", "Type": detectedFileType}).Warnf("Error getting data: %v - %v", err, path)
			data, buf, err = nil, nil, nil
			c.deleteFile(path)
			return
		} else {
			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true}).Infof(string(data.Format.Filename))

			filehash := imohash.New()
			sum, _ := filehash.SumFile(path)

			log.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "File Hashed": true}).Debugf("New File Hash: %x", sum)

			err := c.db.Update(func(tx *bolt.Tx) error {
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
		return
	} else {
		content := http.DetectContentType(buf)
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Debugf("File \"%v\" is of type \"%v\"", path, content)
		buf = nil
		log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Infof("File \"%v\" is not a recongized file type", path)
		ret := c.discord.Notify("Unknown file detected", fmt.Sprintf("\"%v\" is not a Video, Audio, Image, Subtitle, or Plaintext file.", path), "unknowndetected")
		if !ret {
			log.Error("Could not notify Discord")
		}
		c.Stats.UnknownFileCount++
		c.deleteFile(path)
		return
	}
}

func (c *Checkrr) deleteFile(path string) {
	if c.sonarr.MatchPath(path) {
		c.sonarr.RemoveFile(path)
		c.Stats.SonarrSubmissions++
	} else if c.radarr.MatchPath(path) {
		c.radarr.RemoveFile(path)
		c.Stats.RadarrSubmissions++
	} else if c.lidarr.MatchPath(path) {
		c.lidarr.RemoveFile(path)
		c.Stats.LidarrSubmissions++
	} else {
		log.WithFields(log.Fields{"Unknown File": true}).Infof("Couldn't find a target for file \"%v\". File is unknown.", path)
		if c.config.GetBool("removeunknownfiles") {
			e := os.Remove(path)
			if e != nil {
				log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown", "Deleted": false}).Warnf("Could not delete File: \"%v\"", path)
				return
			}
			log.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown", "Deleted": true}).Warnf("Removed File: \"%v\"", path)
			c.discord.Notify("Unknown file deleted", fmt.Sprintf("\"%v\" was removed.", path), "unknowndeleted")
			c.Stats.UnknownFilesDeleted++
			return
		}
	}
}
