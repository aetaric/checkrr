package check

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aetaric/checkrr/logging"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/aetaric/checkrr/connections"
	"github.com/aetaric/checkrr/features"
	"github.com/aetaric/checkrr/hidden"
	"github.com/aetaric/checkrr/notifications"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/kalafut/imohash"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type Checkrr struct {
	Stats              features.Stats
	DB                 *bolt.DB
	Running            bool
	csv                features.CSV
	notifications      notifications.Notifications
	sonarr             []connections.Sonarr
	radarr             []connections.Radarr
	lidarr             []connections.Lidarr
	ignoreExts         []string
	ignorePaths        []string
	removeVideo        []string
	removeAudio        []string
	removeLang         []string
	ignoreHidden       bool
	requireAudio       bool
	ffProbe            bool
	ffMpegFull         bool
	ffMpegQuick        bool
	ffMpegQuickSeconds int64
	FullConfig         *koanf.Koanf
	config             *koanf.Koanf
	Chan               *chan []string
	Logger             *logging.Log
	Localizer          *i18n.Localizer
}

func (c *Checkrr) Run() {

	// Prevent multiple checkrr goroutines from running
	if !c.Running {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CheckDebugMultiRun",
		})
		c.Logger.Debug(message)
		c.Running = true
	} else {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CheckMultiRunError",
		})
		c.Logger.Error(message)
		return
	}

	c.Stats = features.Stats{Log: *c.Logger, DB: c.DB, Localizer: c.Localizer}
	c.Stats.FromConfig(*c.FullConfig.Cut("stats"))

	// Connect to Sonarr, Radarr, and Lidarr
	c.connectServices()

	// Connect to notifications
	c.connectNotifications()

	// Setup CSV writer
	if c.config.String("csvfile") != "" {
		c.csv = features.CSV{FilePath: c.config.String("csvfile"), Log: c.Logger, Localizer: c.Localizer}
		c.csv.Open()
	}

	c.ignoreExts = c.config.Strings("ignoreexts")
	c.ignorePaths = c.config.Strings("ignorepaths")
	c.removeVideo = c.config.Strings("removevideo")
	c.removeAudio = c.config.Strings("removeaudio")
	c.removeLang = c.config.Strings("removelang")
	c.ignoreHidden = c.config.Bool("ignorehidden")
	c.requireAudio = c.config.Bool("requireaudio")
	c.ffMpegFull = c.config.Bool("ffmpeg-full")
	c.ffMpegQuick = c.config.Bool("ffmpeg-quick")
	c.ffMpegQuickSeconds = c.config.Int64("ffmpeg-quick-seconds")
	c.ffProbe = c.config.Bool("ffprobe")

	// warn if ffprobe is disabled and defined flags need it
	if !c.ffProbe {
		if len(c.removeVideo) > 0 {
			c.Logger.Warn("remove video flag is set, but ffprobe is disabled. codec based removal will not run")
		}
		if len(c.removeAudio) > 0 {
			c.Logger.Warn("remove audio flag is set, but ffprobe is disabled. codec based removal will not run")
		}
		if len(c.removeLang) > 0 {
			c.Logger.Warn("remove language flag is set, but ffprobe is disabled. codec based removal will not run")
		}
		if c.requireAudio {
			c.Logger.Warn("require audio flag is set, but ffprobe is disabled. codec based removal will not run")
		}
	}

	// I'm tired of waiting for filetype to support this. We'll force it by adding to the matchers on the fly.
	// TODO: if h2non/filetype#120 ever gets completed, remove this logic
	ts := filetype.AddType("ts", "MPEG-TS")
	m2ts := filetype.AddType("m2ts", "MPEG-TS")
	matchers.Video[ts] = mpegtsMatcher
	matchers.Video[m2ts] = mpegtsMatcher

	c.Stats.Start()

	c.Logger.Debug(c.config.Strings("checkpath"))

	for _, path := range c.config.Strings("checkpath") {
		c.Logger.WithFields(log.Fields{"startup": true}).Debugf("Path: %v", path)

		err := filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "CheckWalkDirError",
					TemplateData: map[string]interface{}{
						"Path": path,
					},
				})
				c.Logger.Warnf(message)
				return err // we need to return here. we will fail all checks otherwise.
			}
			if !d.IsDir() {
				var ignore = false

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
						message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "DBAccessFail",
							TemplateData: map[string]interface{}{
								"Path": err.Error(),
							},
						})
						c.Logger.Fatal(message)
					}

					if hash == nil {
						message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "CheckDebugHashNotFound",
							TemplateData: map[string]interface{}{
								"Path": path,
							},
						})
						c.Logger.WithFields(log.Fields{"DB Hash": "Not Found"}).Debug(message)
						c.checkFile(path)
					} else {
						message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "CheckDebugDBHash",
							TemplateData: map[string]interface{}{
								"Hash": hash,
							},
						})
						c.Logger.WithFields(log.Fields{"DB Hash": "Found"}).Debug(message)

						filehash := imohash.New()
						sum, _ := filehash.SumFile(path)

						message = c.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "CheckDebugFileHash",
							TemplateData: map[string]interface{}{
								"Hash": hex.EncodeToString(sum[:]),
							},
						})
						c.Logger.WithFields(log.Fields{"DB Hash": "Found", "File Hash": "Computed"}).Debug(message)

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
		if err != nil {
			message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "CheckGenericError",
				TemplateData: map[string]interface{}{
					"Error": err.Error(),
				},
			})
			c.Logger.WithFields(log.Fields{"path": path}).Error(message)
		}
	}

	title := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsRunFinishTitle",
	})
	desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsRunFinishDesc",
	})
	c.notifications.Notify(title, desc, "endrun", "")
	c.Stats.Stop()
	c.Stats.Render()
	if c.config.String("csvfile") != "" {
		c.csv.Close()
	}
	c.Running = false
	ch := *c.Chan
	ch <- []string{"time"}
}

func (c *Checkrr) FromConfig(conf *koanf.Koanf) {
	c.config = conf
}

func (c *Checkrr) connectServices() {
	if c.FullConfig.Get("arr") != nil {
		arrConfig := c.FullConfig.Cut("arr")
		arrKeys := c.FullConfig.Cut("arr").Keys()
		for _, key := range arrKeys {
			if strings.Contains(key, "service") {
				k := strings.Split(key, ".")[0]
				config := arrConfig.Cut(k)

				if config.String("service") == "sonarr" {
					sonarr := connections.Sonarr{Log: c.Logger, Localizer: c.Localizer}
					sonarr.FromConfig(config)
					sonarrConnected, sonarrMessage := sonarr.Connect()
					message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ArrConnectField",
						TemplateData: map[string]interface{}{
							"Arr":     k,
							"Service": "Sonarr",
						},
					})
					c.Logger.WithFields(log.Fields{"Startup": true, message: sonarrConnected}).Info(sonarrMessage)
					if sonarrConnected {
						c.sonarr = append(c.sonarr, sonarr)
					}
				}

				if config.String("service") == "radarr" {
					radarr := connections.Radarr{Log: c.Logger, Localizer: c.Localizer}
					radarr.FromConfig(config)
					radarrConnected, radarrMessage := radarr.Connect()
					message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ArrConnectField",
						TemplateData: map[string]interface{}{
							"Arr":     k,
							"Service": "Radarr",
						},
					})
					c.Logger.WithFields(log.Fields{"Startup": true, message: radarrConnected}).Info(radarrMessage)
					if radarrConnected {
						c.radarr = append(c.radarr, radarr)
					}
				}

				if config.String("service") == "lidarr" {
					lidarr := connections.Lidarr{Log: c.Logger, Localizer: c.Localizer}
					lidarr.FromConfig(config)
					lidarrConnected, lidarrMessage := lidarr.Connect()
					message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "ArrConnectField",
						TemplateData: map[string]interface{}{
							"Arr":     k,
							"Service": "Lidarr",
						},
					})
					c.Logger.WithFields(log.Fields{"Startup": true, message: lidarrConnected}).Info(lidarrMessage)
					if lidarrConnected {
						c.lidarr = append(c.lidarr, lidarr)
					}
				}
			}
		}
	}
}

func (c *Checkrr) connectNotifications() {
	if c.FullConfig.Cut("notifications") != nil {
		c.notifications = notifications.Notifications{Log: c.Logger, Localizer: c.Localizer}
		c.notifications.FromConfig(c.FullConfig.Cut("notifications"))
		c.notifications.Connect()
	} else {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NotificationsNone",
		})
		c.Logger.WithFields(log.Fields{"Startup": true, "Notifications Connected": false}).Warn(message)
	}
	title := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsRunStartedTitle",
	})
	desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsRunStartedDesc",
	})
	c.notifications.Notify(title, desc, "startrun", "")
}

func (c *Checkrr) checkFile(path string) {
	ctx := context.Background()

	// This seems like an insane number, but it's only 33KB and will allow detection of all file types via the filetype library
	f, _ := os.Open(path)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "CheckErrorClosing",
				TemplateData: map[string]interface{}{
					"Path":  path,
					"Error": err.Error(),
				},
			})
			c.Logger.WithFields(log.Fields{"fileopen": true}).Warn(message)
		}
	}(f)

	buf := make([]byte, 33000)
	_, err := f.Read(buf)
	if err != nil {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CheckErrorReading",
			TemplateData: map[string]interface{}{
				"Path":  path,
				"Error": err.Error(),
			},
		})
		c.Logger.WithFields(log.Fields{"fileopen": true}).Warn(message)
		return
	}
	var detectedFileType string
	var formatLong string

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
		// FFProbe checks
		if c.ffProbe {
			data, err := ffprobe.ProbeURL(ctx, path)
			if err != nil {
				message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "CheckErrorReading",
					TemplateData: map[string]interface{}{
						"Path":  path,
						"Error": err.Error(),
					},
				})
				c.Logger.WithFields(log.Fields{"FFProbe": "failed", "Type": detectedFileType}).Warn(message)
				c.deleteFile(path, "data problem")
				data, buf, err = nil, nil, nil
				return
			}
			c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true}).Infof(data.Format.Filename)

			c.Logger.Debug(data.Format.FormatName)

			if c.requireAudio {
				hasAudio := false
				for _, stream := range data.Streams {
					if stream.CodecType == "audio" {
						hasAudio = true
						break
					}
				}

				if !hasAudio {
					message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "CheckNoAudioStream",
						TemplateData: map[string]interface{}{
							"Path": path,
						},
					})
					c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "NoAudio": true}).Info(message)
					c.deleteFile(path, "no audio streams")
					return
				}
			}

			if detectedFileType == "Video" {
				for _, stream := range data.Streams {
					c.Logger.Debug(stream.CodecName)
					for _, codec := range c.removeVideo {
						if stream.CodecName == codec {
							message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
								MessageID: "CheckFormatDetected",
								TemplateData: map[string]interface{}{
									"Codec": data.FirstVideoStream().CodecName,
								},
							})
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Info(message)
							c.deleteFile(path, "video codec")
							return
						}
					}
					for _, codec := range c.removeAudio {
						if stream.CodecName == codec {
							message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
								MessageID: "CheckFormatDetected",
								TemplateData: map[string]interface{}{
									"Codec": data.FirstAudioStream().CodecName,
								},
							})
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Info(message)
							c.deleteFile(path, "audio codec")
							return
						}
					}
					for _, language := range c.removeLang {
						streamlang, err := stream.TagList.GetString("Language")
						if err == nil {
							if streamlang == language {
								message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
									MessageID: "CheckFormatDetected",
									TemplateData: map[string]interface{}{
										"Codec": streamlang,
									},
								})
								c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName, "Language": streamlang}).Info(message)
								c.deleteFile(path, "audio lang")
								return
							}
						} else {
							message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
								MessageID: "CheckAudioStreamError",
							})
							c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName, "Language": "unknown"}).Warn(message)
							//c.deleteFile(path, audio lang)
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
								message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
									MessageID: "CheckFormatDetected",
									TemplateData: map[string]interface{}{
										"Codec": data.FirstAudioStream().CodecName,
									},
								})
								c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": stream.CodecName}).Info(message)
								c.deleteFile(path, "audio codec")
								return
							}
						}
					}
				} else {
					message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "CheckAudioStreamMissing",
						TemplateData: map[string]interface{}{
							"Path": path,
						},
					})
					c.Logger.WithFields(log.Fields{"Format": data.Format.FormatLongName, "Type": detectedFileType, "FFProbe": true, "Codec": "unknown"}).Info(message)
					c.deleteFile(path, "no audio in video")
					return
				}
			}
			formatLong = data.Format.FormatLongName
			buf, data = nil, nil
		}

		// FFMPEG quick checks
		if c.ffMpegQuick {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ffMpegQuickSeconds+10)*time.Second)
			defer cancel()

			args := []string{
				"-v", "error",
				"-i", path,
				"-t", fmt.Sprintf("%d", c.ffMpegQuickSeconds),
				"-f", "null", "-",
			}

			c.Logger.WithFields(log.Fields{"FFMPEG-quick": true}).Debugf("Running FFmpeg, Quick %d Seconds", c.ffMpegQuickSeconds)
			out, err := runFFmpeg(ctx, args)
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					// exec failure (ffmpeg missing, killed, etc)
					c.Logger.WithFields(log.Fields{"FFMPEG-quick": true}).Error(err)
					// if ffmpeg errored, we should not trust the output
					return
				}
			}

			// ffmpeg returns stderr lines when corrupt (because -v error)
			if strings.TrimSpace(out) != "" {
				c.deleteFile(path, out)
			}
		}

		// FFMPEG full checks
		if c.ffMpegFull {
			// use background because we want to check the whole file
			ctx := context.Background()

			args := []string{
				"-v", "error",
				"-i", path,
				"-f", "null", "-",
			}
			c.Logger.WithFields(log.Fields{"FFMPEG-Full": true}).Debug("Running FFmpeg, Full")
			out, err := runFFmpeg(ctx, args)
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					// exec failure (ffmpeg missing, killed, etc)
					c.Logger.WithFields(log.Fields{"FFMPEG-Full": true}).Error(err)
					// if ffmpeg errored, we should not trust the output
					return
				}
			}

			// ffmpeg returns stderr lines when corrupt (because -v error)
			if strings.TrimSpace(out) != "" {
				c.deleteFile(path, out)
			}
		}

		// File hashing
		fileHash := imohash.New()
		sum, _ := fileHash.SumFile(path)

		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CheckNewFileHash",
			TemplateData: map[string]interface{}{
				"Hash": sum,
			},
		})
		c.Logger.WithFields(log.Fields{"Format": formatLong, "Type": detectedFileType, "File Hashed": true}).Debug(message)

		err := c.DB.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("Checkrr"))
			err := b.Put([]byte(path), sum[:])
			return err
		})
		if err != nil {
			message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DBFailure",
				TemplateData: map[string]interface{}{
					"Error": err.Error(),
				},
			})
			c.Logger.WithFields(log.Fields{"Format": formatLong, "Type": detectedFileType, "DB Update": "Failure"}).Warn(message)
		}

		return
	} else if filetype.IsImage(buf) || filetype.IsDocument(buf) || http.DetectContentType(buf) == "text/plain; charset=utf-8" {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CheckInvalidFile",
			TemplateData: map[string]interface{}{
				"Path": path,
			},
		})
		c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Other"}).Info(message)
		buf = nil
		c.Stats.NonVideo++
		c.Stats.Write("NonVideo", c.Stats.NonVideo)
		return
	}

	content := http.DetectContentType(buf)
	message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "CheckDebugFileType",
		TemplateData: map[string]interface{}{
			"Path":    path,
			"Content": content,
		},
	})
	c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Debug(message)
	buf = nil

	message = c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "CheckNotRecognized",
		TemplateData: map[string]interface{}{
			"Path": path,
		},
	})
	c.Logger.WithFields(log.Fields{"FFProbe": false, "Type": "Unknown"}).Info(message)

	title := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsUnknownFileTitle",
	})
	desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsUnknownFileDesc",
		TemplateData: map[string]interface{}{
			"Path": path,
		},
	})
	c.notifications.Notify(title, desc, "unknowndetected", path)

	c.Stats.UnknownFileCount++
	c.Stats.Write("UnknownFiles", c.Stats.UnknownFileCount)
	c.deleteFile(path, "not recognized")
	return
}

func (c *Checkrr) deleteFile(path string, reason string) {
	title := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "NotificationsReacquireTitle",
	})
	for _, sonarr := range c.sonarr {
		if sonarr.Process && sonarr.MatchPath(path) {
			sonarr.RemoveFile(path)
			desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsReacquireDesc",
				TemplateData: map[string]interface{}{
					"Path":    path,
					"Service": "sonarr",
				},
			})
			c.notifications.Notify(title, desc, "reacquire", path)
			c.Stats.SonarrSubmissions++
			c.Stats.Write("Sonarr", c.Stats.SonarrSubmissions)
			c.recordBadFile(path, "sonarr", reason)
			return
		}
	}
	for _, radarr := range c.radarr {
		if radarr.Process && radarr.MatchPath(path) {
			radarr.RemoveFile(path)
			desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsReacquireDesc",
				TemplateData: map[string]interface{}{
					"Path":    path,
					"Service": "radarr",
				},
			})
			c.notifications.Notify(title, desc, "reacquire", path)
			c.Stats.RadarrSubmissions++
			c.Stats.Write("Radarr", c.Stats.RadarrSubmissions)
			c.recordBadFile(path, "radarr", reason)
			return
		}
	}
	for _, lidarr := range c.lidarr {
		if lidarr.Process && lidarr.MatchPath(path) {
			lidarr.RemoveFile(path)
			desc := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "NotificationsReacquireDesc",
				TemplateData: map[string]interface{}{
					"Path":    path,
					"Service": "lidarr",
				},
			})
			c.notifications.Notify(title, desc, "reacquire", path)
			c.Stats.LidarrSubmissions++
			c.Stats.Write("Lidarr", c.Stats.LidarrSubmissions)
			c.recordBadFile(path, "lidarr", reason)
			return
		}
	}
	message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
		MessageID: "CheckUnknownFile",
		TemplateData: map[string]interface{}{
			"Path": path,
		},
	})
	c.Logger.WithFields(log.Fields{"Unknown File": true}).Info(message)
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
		}
		return nil
	})

	if err != nil {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "DBFailure",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		c.Logger.WithFields(log.Fields{"DB Update": "Failure"}).Warn(message)
	}
	if len(c.config.String("csvfile")) > 0 {
		log.Debug("writting bad file to csv")
		c.csv.Write(path, fileType)
	}
}

func runFFmpeg(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	stderrBytes, readErr := io.ReadAll(stderrPipe)

	waitErr := cmd.Wait()

	if readErr != nil {
		return "", readErr
	}

	var firstLine string
	if len(stderrBytes) > 0 {
		lines := strings.SplitN(string(stderrBytes), "\n", 2)
		firstLine = lines[0]
	}

	if ctx.Err() != nil {
		return firstLine, ctx.Err()
	}

	return firstLine, waitErr
}

type BadFile struct {
	FileExt   string `json:"fileExt"`
	Reacquire bool   `json:"reacquire"`
	Service   string `json:"service"`
	Date      int64  `json:"date"`
	Reason    string `json:"reason"`
}

// TODO: if h2non/filetype#120 ever gets completed, remove this logic
func mpegtsMatcher(buf []byte) bool {
	return len(buf) > 1 &&
		buf[0] == 0x47
}
