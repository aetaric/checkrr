package features

import (
	"encoding/csv"
	"os"

	"github.com/aetaric/checkrr/logging"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	log "github.com/sirupsen/logrus"
)

type CSV struct {
	FilePath   string
	fileHandle *os.File
	fileWriter *csv.Writer
	Log        *logging.Log
	Localizer  *i18n.Localizer
}

func (c *CSV) Open() {
	var err error
	c.fileHandle, err = os.OpenFile(c.FilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CSVFileCreateFailed",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		c.Log.WithFields(log.Fields{"startup": true}).Fatalf(message)
	}
	c.fileWriter = csv.NewWriter(c.fileHandle)
}

func (c *CSV) Write(path string, t string) {
	c.fileWriter.Write([]string{path, t})
	c.fileWriter.Flush()
	c.Log.Debug("wrote csv entry")
}

func (c *CSV) Close() {
	c.fileWriter.Flush()
	c.fileHandle.Sync()
	c.fileHandle.Close()
	c.Log.Debug("closed csv file")
}
