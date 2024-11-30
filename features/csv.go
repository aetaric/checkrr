package features

import (
	"encoding/csv"
	"github.com/aetaric/checkrr/logging"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"os"

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
	c.fileHandle, err = os.Create(c.FilePath)
	if err != nil {
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "CSVFileCreateFailed",
			TemplateData: map[string]interface{}{
				"Error": err.Error(),
			},
		})
		c.Log.WithFields(log.Fields{"startup": true}).Fatalf(message)
	}
	defer c.fileHandle.Close()
	c.fileWriter = csv.NewWriter(c.fileHandle)
	defer c.fileWriter.Flush()
}

func (c *CSV) Write(path string, t string) {
	c.fileWriter.Write([]string{path, t})
}
