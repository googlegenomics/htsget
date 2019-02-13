package file

import (
	"io/ioutil"
	"os"

	"github.com/googlegenomics/htsget/sources/file"

	"github.com/googlegenomics/htsget/block"

	"github.com/gin-gonic/gin"
	"github.com/googlegenomics/htsget/htsget-multisource-server/utils"
)

//NewBlockHandler takes in a directory and returns a handler that returns a block
func NewBlockHandler(directory string) func(c *gin.Context) {
	return func(c *gin.Context) {

		if err := utils.ParseFormat(c.Query("format")); err != nil {
			c.String(400, "Unsupported Format")
			return
		}

		chunk, id, err := utils.HTSGETParams(map[string]string{
			"start": c.Query("start"),
			"end":   c.Query("end"),
			"id":    c.Param("id"),
		})

		if err != nil {
			c.String(400, "Error parsing params")
		}

		f, err := os.Open(directory + "/" + id + ".bam")

		if err != nil {
			c.String(400, "Error finding the file")
			return
		}
		defer f.Close()

		readCloser, err := block.ReadBlock(file.NewFileRangeReader(*f), chunk)
		if err != nil {
			c.String(400, "Error parsing file")
			return
		}
		defer readCloser.Close()

		all, err := ioutil.ReadAll(readCloser)
		if err != nil {
			c.String(400, "Error reading file")
			return
		}
		c.Header("Content-Type", "application/octet-stream")
		_, err = c.Writer.Write(all)

	}
}
