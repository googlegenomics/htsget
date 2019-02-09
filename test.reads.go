package main

import (
	"fmt"
	"math"
	"os"

	"github.com/googlegenomics/htsget/internal/genomics"
	"github.com/googlegenomics/htsget/reads"
)

// func main() {
// 	testReads()
// }

func testReads() {
	p := fmt.Println
	bam1, _ := os.Open("/Users/aaliomer/github/htsget/api/testdata/wgs_bam_NA12878_20k_b37_NA12878.bam.bai")

	defer bam1.Close() // bam2, _ := gzip.NewReader(bam1)

	// file, _ := ioutil.ReadAll(bam2)

	// for i := 0; i <= 21; i++ {
	reference, err := reads.Chunks(bam1, genomics.Region{
		Start:       0,
		End:         math.MaxUint32,
		ReferenceID: int32(1),
	}, 1024*1024*1024)
	if err != nil {
		p(err)
		return
	}
	for i := range reference {
		c1 := reference[i]
		p(uint64(c1.Start), uint64(c1.End), uint64(c1.End-c1.Start))
	}
	// }

}
