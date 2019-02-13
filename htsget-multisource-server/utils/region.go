package utils

import (
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/genomics"
)

func ParseRegion(query url.Values, data io.Reader) (genomics.Region, error) {
	var (
		name  = query.Get("referenceName")
		start = query.Get("start")
		end   = query.Get("end")
	)
	if name == "" && start == "" && end == "" {
		return genomics.AllMappedReads, nil
	}
	if name == "" {
		return genomics.Region{}, fmt.Errorf("Missing Reference Name")
	}

	id, err := bam.GetReferenceID(data, name)
	if err != nil {
		return genomics.Region{}, fmt.Errorf("resolving reference %q: %v", name, err)
	}

	region := genomics.Region{ReferenceID: id}

	if start != "" {
		n, err := strconv.ParseUint(start, 10, 32)
		if err != nil {
			return genomics.Region{}, fmt.Errorf("parsing start: %v", err)
		}
		region.Start = uint32(n)
	}

	if end != "" {
		n, err := strconv.ParseUint(end, 10, 32)
		if err != nil {
			return genomics.Region{}, fmt.Errorf("parsing end: %v", err)
		}
		region.End = uint32(n)
	}

	return region, nil
}
