package file

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/googlegenomics/htsget/htsget-multisource-server/model"

	"github.com/googlegenomics/htsget/reads"

	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/genomics"

	"github.com/gin-gonic/gin"
	"github.com/googlegenomics/htsget/htsget-multisource-server/utils"
)

//NewReadsHandler builds a gin handler
func NewReadsHandler(directory string, blockSize uint64, baseURL string) func(c *gin.Context) {
	return func(c *gin.Context) {
		chunk, id, err := utils.HTSGETParams(map[string]string{
			"start": c.Query("start"),
			"end":   c.Query("end"),
			"id":    c.Param("id"),
		})

		if err != nil {
			c.String(400, "Error parsing params")
		}

		f1, err := os.Open(directory + "/" + id + ".bam")

		if err != nil {
			c.String(400, "Error finding the file")
			return
		}
		defer f1.Close()

		ref, err := bam.GetReferenceID(f1, c.Query("referenceName"))
		if err != nil {
			c.String(400, "Error processing reference name")
			return
		}
		f, err := os.Open(directory + "/" + id + ".bam.bai")

		if err != nil {
			c.String(400, "Error finding the file")
			return
		}
		defer f.Close()
		chunks, err := reads.Chunks(f, genomics.Region{
			ReferenceID: ref,
			Start:       uint32(chunk.Start),
			End:         uint32(chunk.End),
		}, blockSize)

		if err != nil {
			c.String(400, "Error processing reference name")
			return
		}

		htsget := model.HTSGetResponse{}
		htsget.Htsget.Format = "BAM"
		htsget.Htsget.Urls = make([]model.URL, len(chunks))

		for i, c := range chunks {
			if c != nil {
				start := strconv.FormatUint(uint64(c.Start), 10)
				end := strconv.FormatUint(uint64(c.End), 10)
				htsget.Htsget.Urls[i] = model.URL{
					Url: baseURL + "/block/" + id + "?start=" + start + "&end=" + end,
				}
			}
		}

		enc := json.NewEncoder(c.Writer)
		enc.SetEscapeHTML(false)
		c.Header("Content-Type", "application/json")
		c.Status(200)
		err = enc.Encode(&htsget)
		if err != nil {
			c.String(400, "Error generating result")
			return
		}

	}
}
