package check

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aetaric/checkrr/connections"
	"github.com/aetaric/checkrr/features"
	"github.com/aetaric/checkrr/hidden"
	"github.com/aetaric/checkrr/notifications"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
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
	sonarr        []connections.Sonarr
	radarr        []connections.Radarr
	lidarr        []connections.Lidarr
	ignoreExts    []string
	ignorePaths   []string
	removeVideo   []string
	removeAudio   []string
	removeLang    []string
	ignoreHidden  bool
	config        *viper.Viper
	FullConfig    *viper.Viper
	Chan          *chan []string
	Logger        *logging.Log
}

func (c *Checkrr) Run() {

	// Prevent multiple checkrr goroutines from running
	if !c.Running {
		c.Logger.Debug("Setting Lock to prevent multi-runs")
		c.Running = true
	} else {
		c.Logger.Error("Tried to run more than one check at a time. Adjust your cron timing. If this is your first run, use --run-once.")
		return
	}

	c.Stats = features.Stats{Log: *c.Logger, DB: c.DB}

	if c.FullConfig.Sub("stats") != nil {
		c.Stats.FromConfig(*c.FullConfig.Sub("stats"))
	}

	// Connect to Sonarr, Radarr, and Lidarr
	c.connectServices()

	// Connect to notifications
	c.connectNotifications()

	// Setup CSV writer
	if c.config.GetString("csvfile") != "" {
		c.csv = features.CSV{FilePath: c.config.GetString("csvfile"), Log: c.Logger}
		c.csv.Open()
	}

	c.ignoreExts = c.config.GetStringSlice("ignoreexts")
	c.ignorePaths = c.config.GetStringSlice("ignorepaths")
	c.removeVideo = c.config.GetStringSlice("removevideo")
	c.removeAudio = c.config.GetStringSlice("removeaudio")
	c.removeLang = c.config.GetStringSlice("removelang")
	c.ignoreHidden = c.config.GetBool("ignorehidden")

	// I'm tired of waiting for filetype to support this. We'll force it by adding to the matchers on the fly.
	// TODO: if h2non/filetype#120 ever gets completed, remove this logic
	ts := filetype.AddType("ts", "MPEG-TS")
	m2ts := filetype.AddType("m2ts", "MPEG-TS")
	matchers.Video[ts] = mpegts_matcher
	matchers.Video[m2ts] = mpegts_matcher

	c.Stats.Start()

	c.Logger.Debug(c.config.GetStringSlice("checkpath"))

	for _, path := range c.config.GetStringSlice("checkpath") {
		c.Logger.WithFields(log.Fields{"startup": true}).Debugf("Path: %v", path)

		filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				c.Logger.Fatalf(err.Error()+" %v", path)
				return err
			}
			if !d.IsDir() {
				var ignore bool = false

				ext := filepath.Ext(path)
				for _, v := range c.ignoreExts {
					if strings.EqualFold(v, ext) {
						ignore = true
					}
				}

				if c.ignoreHidden {
					i, _ := hidden.IsHidden(path)
					if !ignore {
						ignore = i
					}
				}

				for _, v := range c.ignorePaths {
					if strings.Contains(path, v) {
						if !ignore {
							ignore = true
						}
					}
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
						c.Logger.Fatalf("Error accessing database: %v", err.Error())
					}

					if hash == nil {
						c.Logger.WithFields(log.Fields{"DB Hash": "Not Found"}).Debugf("DB Hash not found, checking file \"%s\"", path)
						c.checkFile(path)
					} else {
						c.Logger.WithFields(log.Fields{"DB Hash": "Found"}).Debugf("DB Hash: %x", hash)

						filehash := imohash.New()
						sum, _ := filehash.SumFile(path)

						c.Logger.WithFields(log.Fields{"DB Hash": "Found", "File Hash": "Computed"}).Debugf("File Hash: %x", hex.EncodeToString(sum[:]))

						if hex.EncodeToString(sum[:]) != hex.EncodeToString(hash[:]) {
							c.Logger.WithFields(log.Fields{"Hash Match": false}).Infof("\"%v\"", path)
							c.Stats.HashMismatches++
							c.Stats.Write("HashMismatches", c.Stats.HashMismatches)
							c.checkFile(path)
						} else {
							c.Logger.WithFields(log.Fields{"Hash Match": true}).Infof("\"%v\"", path)
							c.Stats.HashMatches++
							c.Stats.Write("HashMatches", c.Stats.HashMatches)
						}
					}
				} else {
					c.Logger.WithFields(log.Fields{"Ignored": true}).Debugf("\"%s\"", path)
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
	if viper.GetViper().Sub("arr") != nil {
		arrConfig := viper.GetViper().Sub("arr")
		arrKeys := viper.GetViper().Sub("arr").AllKeys()
		for _, key := range arrKeys {
			if strings.Contains(key, "service") {
				k := strings.Split(key, ".")[0]
				config := arrConfig.Sub(k)

				if config.GetString("service") == "sonarr" {
					sonarr := connections.Sonarr{Log: c.Logger}
					sonarr.FromConfig(config)
					sonarrConnected, sonarrMessage := sonarr.Connect()
					c.Logger.WithFields(log.Fields{"Startup": true, fmt.Sprintf("Sonarr \"%s\" Connected", k): sonarrConnected}).Info(sonarrMessage)
					if sonarrConnected {
						c.sonarr = append(c.sonarr, sonarr)
					}
				}

				if config.GetString("service") == "radarr" {
					radarr := connections.Radarr{Log: c.Logger}
					radarr.FromConfig(config)
					radarrConnected, radarrMessage := radarr.Connect()
					c.Logger.WithFields(log.Fields{"Startup": true, fmt.Sprintf("Radarr \"%s\" Connected", k): radarrConnected}).Info(radarrMessage)
					if radarrConnected {
						c.radarr = append(c.radarr, radarr)
					}
				}

				if config.GetString("service") == "lidarr" {
					lidarr := connections.Lidarr{Log: c.Logger}
					lidarr.FromConfig(config)
					lidarrConnected, lidarrMessage := lidarr.Connect()
					c.Logger.WithFields(log.Fields{"Startup": true, fmt.Sprintf("Lidarr \"%s\" Connected", k): lidarrConnected}).Info(lidarrMessage)
					if lidarrConnected {
						c.lidarr = append(c.lidarr, lidarr)
					}
				}
			}
		}
	}
}

func (c *Checkrr) connectNotifications() {
	if viper.GetViper().Sub("notifications") != nil {
		c.notifications = notifications.Notifications{Log: c.Logger}
		c.notifications.FromConfig(*viper.GetViper().Sub("notifications"))
		c.notifications.Connect()
	} else {
		c.Logger.WithFields(log.Fields{"Startup": true, "Notifications Connected": false}).Warn("No config options for notifications found.")
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
			c.Logger.WithFields(log.Fields{"FFProbe": "failed", "Type": detectedFileType}).Warnf("Error getting data: %v - %v", err, path)
			c.deleteFile(path, "data problem")
			data, buf, err = nil, nil, nil
			return
		} else {
			c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true}).Infof(string(data.Format.Filename))

			c.Logger.Debug(data.Format.FormatName)

			if detectedFileType == "Video" {
				for _, stream := range data.Streams {
					c.Logger.Debug(stream.CodecName)
					for _, codec := range c.removeVideo {
						if stream.CodecName == codec {
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Infof("Detected %s. Removing.", string(data.FirstVideoStream().CodecName))
							c.deleteFile(path, "video codec")
							return
						}
					}
					for _, codec := range c.removeAudio {
						if stream.CodecName == codec {
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Infof("Detected %s. Removing.", string(data.FirstVideoStream().CodecName))
							c.deleteFile(path, "audio codec")
							return
						}
					}
					for _, language := range c.removeLang {
						streamlang, err := stream.TagList.GetString("Language")
						if err == nil {
							if streamlang == language {
								c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName, "Language": streamlang}).Infof("Detected %s. Removing.", string(streamlang))
								c.deleteFile(path, "audio lang")
								return
							}
						} else {
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName, "Language": "unknown"}).Warn("Error getting audio stream language")
							//c.deleteFile(path, audio lang")
						}
					}
				}
			} else {
				if data.FirstAudioStream() != nil {
					c.Logger.Debug(data.FirstAudioStream().CodecName)
					for _, stream := range data.Streams {
						c.Logger.Debug(stream.CodecName)
						for _, codec := range c.removeAudio {
							if stream.CodecName == codec {
								c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Infof("Detected %s. Removing.", string(data.FirstVideoStream().CodecName))
								c.deleteFile(path, "audio codec")
								return
							}
						}
					}
				} else {
					c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": "unknown"}).Infof("No Audio Stream detected for audio file: %s. Removing.", string(path))
					c.deleteFile(path, "no audio in video")
					return
				}
			}

			filehash := imohash.New()
			sum, _ := filehash.SumFile(path)

			c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "File Hashed": true}).Debugf("New File Hash: %x", sum)

			err := c.DB.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("Checkrr"))
				err := b.Put([]byte(path), sum[:])
				return err
			})
			if err != nil {
				c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "DB Update": "Failure"}).Warnf("Error: %v", err.Error())
			}

			buf, data = nil, nil
			return
		}
	} else if filetype.IsImage(buf) || filetype.IsDocument(buf) || http.DetectContentType(buf) == "text/plain; charset=utf-8" {
		c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Other"}).Infof("File \"%v\" is an image or subtitle file, skipping...", path)
		buf = nil
		c.Stats.NonVideo++
		c.Stats.Write("NonVideo", c.Stats.NonVideo)
		return
	} else {
		content := http.DetectContentType(buf)
		c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Debugf("File \"%v\" is of type \"%v\"", path, content)
		buf = nil
		c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Infof("File \"%v\" is not a recognized file type", path)
		c.notifications.Notify("Unknown file detected", fmt.Sprintf("\"%v\" is not a Video, Audio, Image, Subtitle, or Plaintext file.", path), "unknowndetected", path)
		c.Stats.UnknownFileCount++
		c.Stats.Write("UnknownFiles", c.Stats.UnknownFileCount)
		c.deleteFile(path, "not recognized")
		return
	}
}

func (c *Checkrr) deleteFile(path string, reason string) {
	for _, sonarr := range c.sonarr {
		if sonarr.Process && sonarr.MatchPath(path) {
			sonarr.RemoveFile(path)
			c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to sonarr to be reacquired", path), "reacquire", path)
			c.Stats.SonarrSubmissions++
			c.Stats.Write("Sonarr", c.Stats.SonarrSubmissions)
			c.recordBadFile(path, "sonarr", reason)
			return
		}
	}
	for _, radarr := range c.radarr {
		if radarr.Process && radarr.MatchPath(path) {
			radarr.RemoveFile(path)
			c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to radarr to be reacquired", path), "reacquire", path)
			c.Stats.RadarrSubmissions++
			c.Stats.Write("Radarr", c.Stats.RadarrSubmissions)
			c.recordBadFile(path, "radarr", reason)
			return
		}
	}
	for _, lidarr := range c.lidarr {
		if lidarr.Process && lidarr.MatchPath(path) {
			lidarr.RemoveFile(path)
			c.notifications.Notify("File Reacquire", fmt.Sprintf("\"%v\" was sent to lidarr to be reacquired", path), "reacquire", path)
			c.Stats.LidarrSubmissions++
			c.Stats.Write("Lidarr", c.Stats.LidarrSubmissions)
			c.recordBadFile(path, "lidarr", reason)
			return
		}
	}
	c.Logger.WithFields(log.Fields{"Unknown File": true}).Infof("Couldn't find a target for file \"%v\". File is unknown.", path)
	c.recordBadFile(path, "unknown", reason)
}

func (c *Checkrr) recordBadFile(path string, fileType string, reason string) {

	bad := BadFile{}
	if fileType != "unknown" {
		bad.Reacquire = true
	} else {
		bad.Reacquire = false
	}

	bad.Service = fileType
	bad.FileExt = filepath.Ext(path)
	bad.Date = time.Now().UTC().Unix() // put this in UTC for the webui to render in local later
	bad.Reason = reason

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
		c.Logger.WithFields(log.Fields{"DB Update": "Failure"}).Warnf("Error: %v", err.Error())
	}

	if c.config.GetString("csvfile") != "" {
		c.csv.Write(path, fileType)
	}
}

type BadFile struct {
	FileExt   string `json:"fileExt"`
	Reacquire bool   `json:"reacquire"`
	Service   string `json:"service"`
	Date      int64  `json:"date"`
	Reason    string `json:"reason"`
}

// TODO: if h2non/filetype#120 ever gets completed, remove this logic
func mpegts_matcher(buf []byte) bool {
	return len(buf) > 1 &&
		buf[0] == 0x47
}
