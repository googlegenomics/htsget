package file

import (
	"os"

	"github.com/googlegenomics/htsget/reads"

	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/genomics"

	"github.com/gin-gonic/gin"
	"github.com/googlegenomics/htsget/htsget-multisource-server/utils"
)

func NewReadsHandler(directory string, blockSize uint64, baseURl string) func(c *gin.Context) {
	return func(c *gin.Context) {
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

		ref, err := bam.GetReferenceID(f, c.Param("referenceName"))
		if err != nil {
			c.String(400, "Error processing reference name")
			return
		}
		f, err = os.Open(directory + "/" + id + ".bam.bai")

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

		urls := make([]gin.H, len(chunks))

		for _, c := range chunks {
			urls = append(urls, gin.H{
				//TODO fix this thing
				"url": baseURl + "/block/" + id + "?start=" + string(c.Start) + "&end=" + string(c.End),
			})
		}
		c.JSON(200, gin.H{
			"htsget": gin.H{
				"format": "BAM",
				"urls":   urls,
			},
		})
	}
}
