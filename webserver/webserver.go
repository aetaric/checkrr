package webserver

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/aetaric/checkrr/logging"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aetaric/checkrr/check"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/knadh/koanf/v2"
	"github.com/robfig/cron/v3"
	bolt "go.etcd.io/bbolt"
)

//go:embed build
var staticFS embed.FS

var fileInfo [][]string

var baseurl BaseURL

var db *bolt.DB
var scheduler *cron.Cron
var cronEntry cron.EntryID
var checkrrInstance *check.Checkrr
var checkrrLogger *logging.Log

type Webserver struct {
	Port           int
	BaseURL        BaseURL
	tls            bool
	cert           string
	key            string
	data           chan []string
	trustedProxies []string
	DB             *bolt.DB
	config         *koanf.Koanf
	FullConfig     *koanf.Koanf
}

type BaseURL string

// EnforceTrailingSlash ensures that the base URL has a trailing slash.
func (b BaseURL) EnforceTrailingSlash() BaseURL {
	if !strings.HasSuffix(string(b), "/") {
		return BaseURL(string(b) + "/")
	}
	return b
}

// String returns the base URL as a string.
func (b BaseURL) String() string {
	return string(b)
}

func (w *Webserver) FromConfig(conf *koanf.Koanf, c chan []string, checkrr *check.Checkrr) {
	w.config = conf
	w.Port = conf.Int("port")
	w.tls = conf.Bool("tls")
	fmt.Println(w.tls)
	if w.tls {
		w.key = conf.String("certs.key")
		w.cert = conf.String("certs.cert")
	}
	w.BaseURL = BaseURL(conf.String("baseurl")).EnforceTrailingSlash()
	baseurl = w.BaseURL
	if conf.Strings("trustedproxies") != nil {
		w.trustedProxies = conf.Strings("trustedproxies")
	} else {
		w.trustedProxies = nil
	}
	w.data = c
	db = w.DB
	checkrrInstance = checkrr
	checkrrLogger = checkrr.Logger
}

func (w *Webserver) AddScehduler(cron *cron.Cron, entryid cron.EntryID) {
	scheduler = cron
	cronEntry = entryid
}

func (w *Webserver) Run() {
	// Build a waitgroup so we can have a webserver and a chan processor
	wg := new(sync.WaitGroup)
	wg.Add(2)
	// Webserver
	go func() {
		createServer(w)
		wg.Done()
	}()
	// chan processor
	go func() {
		for data := range w.data {
			fileInfo = append(fileInfo, data)
		}
		wg.Done()
	}()

	wg.Wait()
}

func createServer(w *Webserver) *gin.Engine {
	embeddedBuildFolder := newStaticFileSystem()
	// use debug mode if chekrr.debug is true
	if w.FullConfig.Bool("checkrr.debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	err := router.SetTrustedProxies(w.trustedProxies)
	if err != nil {
		checkrrLogger.Warn("Error setting Trusted Proxies")
	}
	router.Use(static.Serve(w.BaseURL.String(), embeddedBuildFolder))
	api := router.Group(w.BaseURL.String() + "api")
	api.GET("/files/bad", getBadFiles)
	api.POST("/files/bad", deleteBadFiles)
	api.GET("/stats/current", getCurrentStats)
	api.GET("/stats/historical", getHistoricalStats)
	api.GET("/schedule", getSchedule)
	api.POST("/run", runCheckrr)

	if w.tls {
		checkrrLogger.Infof("Starting HTTPS Webserver on port %d", w.Port)
		err := router.RunTLS(fmt.Sprintf(":%v", w.Port), w.cert, w.key)
		if err != nil {
			checkrrLogger.Warn("Failed to start Webserver in TLS mode")
		}
	} else {
		checkrrLogger.Infof("Starting HTTP Webserver on port %d", w.Port)
		err := router.Run(fmt.Sprintf(":%v", w.Port))
		if err != nil {
			checkrrLogger.Warn("Failed to start Webserver")
		}
	}
	return router
}

func getBadFiles(ctx *gin.Context) {
	var files []badFileData

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-files"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			bad := check.BadFile{}
			json.Unmarshal(v, &bad)
			badfiledata := badFileData{Path: string(k), Data: &bad}
			files = append(files, badfiledata)
		}
		return nil
	})
	if err != nil {
		checkrrLogger.Fatalf("Error accessing database: %v", err.Error())
	}
	ctx.JSON(200, files)
}

func deleteBadFiles(ctx *gin.Context) {
	var files []badFileData
	var postData []int
	ctx.BindJSON(&postData)

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-files"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			bad := check.BadFile{}
			json.Unmarshal(v, &bad)
			badfiledata := badFileData{Path: string(k), Data: &bad}
			files = append(files, badfiledata)
		}
		return nil
	})
	if err != nil {
		checkrrLogger.Fatalf("Error accessing database: %v", err.Error())
	}

	for _, v := range postData {
		db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("Checkrr-files"))
			b.Delete([]byte(files[v-1].Path))
			return nil
		})
	}
	ctx.JSON(200, files)
}

func getCurrentStats(ctx *gin.Context) {
	var stats *Stats
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		statdata := b.Get([]byte("current-stats"))
		s := Stats{}
		json.Unmarshal(statdata, &s)
		stats = &s
		return nil
	})
	if err != nil {
		checkrrLogger.Fatalf("Error accessing database: %v", err.Error())
	}
	ctx.JSON(200, stats)
}

func getHistoricalStats(ctx *gin.Context) {
	var stats []statData
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Checkrr-stats"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			s := Stats{}
			json.Unmarshal(v, &s)
			stat := statData{Timestamp: string(k), Data: &s}
			stats = append(stats, stat)
		}
		return nil
	})
	for len(stats) > 30 {
		_, stats = stats[0], stats[1:]
	}
	if err != nil {
		checkrrLogger.Fatalf("Error accessing database: %v", err.Error())
	}
	ctx.JSON(200, stats)
}

func getSchedule(ctx *gin.Context) {
	if scheduler != nil {
		nextRun := scheduler.Entry(cronEntry).Next.UTC().String()
		ctx.JSON(200, nextRun)
	} else {
		ctx.JSON(200, nil)
	}
}

func runCheckrr(ctx *gin.Context) {
	go checkrrInstance.Run()
	ctx.JSON(200, nil)
}

// file system code
type staticFileSystem struct {
	http.FileSystem
}

var _ static.ServeFileSystem = (*staticFileSystem)(nil)

func newStaticFileSystem() *staticFileSystem {
	sub, err := fs.Sub(staticFS, "build")

	if err != nil {
		panic(err)
	}

	return &staticFileSystem{
		FileSystem: http.FS(sub),
	}
}

func (s *staticFileSystem) Exists(prefix string, path string) bool {
	buildpath := ""
	if baseurl == "/" {
		buildpath = fmt.Sprintf("build%s", path)
	} else {
		buildpath = fmt.Sprintf("build/%s", strings.TrimPrefix(path, baseurl.String()))
	}

	// support for folders
	if strings.HasSuffix(path, "/") {
		_, err := staticFS.ReadDir(strings.TrimSuffix(buildpath, "/"))
		return err == nil
	}

	// support for files
	f, err := staticFS.Open(buildpath)
	if f != nil {
		_ = f.Close()
	}
	return err == nil
}

type statData struct {
	Timestamp string
	Data      *Stats
}

type badFileData struct {
	Path string
	Data *check.BadFile
}

type Stats struct {
	SonarrSubmissions uint64        `json:"sonarrSubmissions"`
	RadarrSubmissions uint64        `json:"radarrSubmissions"`
	LidarrSubmissions uint64        `json:"lidarrSubmissions"`
	FilesChecked      uint64        `json:"filesChecked"`
	HashMatches       uint64        `json:"hashMatches"`
	HashMismatches    uint64        `json:"hashMismatches"`
	VideoFiles        uint64        `json:"videoFiles"`
	AudioFiles        uint64        `json:"audioFiles"`
	UnknownFileCount  uint64        `json:"unknownFileCount"`
	NonVideo          uint64        `json:"nonVideo"`
	Running           bool          `json:"running"`
	Diff              time.Duration `json:"timeDiff"`
}
