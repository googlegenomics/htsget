package reads

import (
	"io"

	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/genomics"
)

func Chunks(bai io.Reader, r genomics.Region, blockSize uint64) ([]*bgzf.Chunk, error) {
	reference, err := bam.Read(bai, r)
	if err != nil {
		return nil, err
	}

	//TODO update block size limit
	reference = bgzf.Merge(reference, blockSize)
	return reference, nil

}
