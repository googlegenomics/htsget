package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/googlegenomics/htsget/block"
	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/source/file"
)

const testFile = "/Users/aaliomer/github/htsget/api/testdata/wgs_bam_NA12878_20k_b37_NA12878"
const testOutput = "/Users/aaliomer/github/htsget/out.bam"

func readblocks() {
	chunks := []bgzf.Chunk{
		bgzf.Chunk{
			Start: 0,
			End:   14705,
		},
		bgzf.Chunk{
			Start: 76393545728,
			End:   153476661248,
		},
	}

	f, err := os.Open(testFile + ".bam")

	out, err := os.Create(testOutput)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, v := range chunks {
		if err != nil {
			fmt.Println(err)
			return
		}
		r, err := block.ReadBlock(file.NewFileRangeReader(*f), v)

		if err != nil {
			fmt.Println(err)
			return
		}
		all, err := ioutil.ReadAll(r)
		if err != nil {
			fmt.Println(err)
			return
		}
		out.Write(all)
	}
	f.Close()
	out.Close()

}
func main() {
	readblocks()
}
