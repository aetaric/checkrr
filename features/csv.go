package features

import (
	"encoding/csv"
	"github.com/aetaric/checkrr/logging"
	"os"

	log "github.com/sirupsen/logrus"
)

type CSV struct {
	FilePath   string
	fileHandle *os.File
	fileWriter *csv.Writer
	Log        *logging.Log
}

func (c *CSV) Open() {
	var err error
	c.fileHandle, err = os.Create(c.FilePath)
	if err != nil {
		c.Log.WithFields(log.Fields{"startup": true}).Fatalf("failed creating file: %s", err)
	}
	defer c.fileHandle.Close()
	c.fileWriter = csv.NewWriter(c.fileHandle)
	defer c.fileWriter.Flush()
}

func (c *CSV) Write(path string, t string) {
	c.fileWriter.Write([]string{path, t})
}
