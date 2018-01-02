package cram

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/googlegenomics/htsget/internal/genomics"
)

// Index holds the data from a CRAM index file (.crai).
type Index struct {
	entries []indexEntry
	// containers maps the file offset of each container to its end.
	containers map[uint64]uint64
}

type indexEntry struct {
	SequenceID      int32
	AlignmentStart  uint32
	AlignmentLength uint32
	ContainerStart  uint64
}

// ReadIndex parses a CRAM index file.
func ReadIndex(r io.Reader) (*Index, error) {
	r, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("ungzipping index: %v", err)
	}

	var index Index
	var containers []uint64
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 6 {
			return nil, fmt.Errorf("wrong number of columns.  Got: %d, want: 6", len(fields))
		}

		var ie indexEntry
		s, err := strconv.ParseInt(fields[0], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing sequence ID: %v", err)
		}
		ie.SequenceID = int32(s)

		ie.AlignmentStart, err = parseUint32(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parsing alignment start: %v", err)
		}

		ie.AlignmentLength, err = parseUint32(fields[2])
		if err != nil {
			return nil, fmt.Errorf("parsing alignment length: %v", err)
		}

		ie.ContainerStart, err = strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing alignment start: %v", err)
		}

		index.entries = append(index.entries, ie)
		containers = append(containers, ie.ContainerStart)
	}

	index.containers = make(map[uint64]uint64)
	var prev uint64
	for _, c := range containers {
		index.containers[prev] = c
		prev = c
	}
	index.containers[prev] = math.MaxUint64

	return &index, nil
}

// GetChunksForRegion returns all chunks that match the specified region. The
// header chunk is always returned.
func (index Index) GetChunksForRegion(region genomics.Region) []*Chunk {
	if region.End == 0 {
		region.End = math.MaxUint32
	}

	chunks := []*Chunk{&Chunk{0, index.containers[0]}}
	for _, ie := range index.entries {
		if region.ReferenceID >= 0 && region.ReferenceID != ie.SequenceID {
			continue
		}
		if region.End < ie.AlignmentStart || region.Start > ie.AlignmentStart+ie.AlignmentLength {
			continue
		}

		chunks = append(chunks, &Chunk{ie.ContainerStart, index.containers[ie.ContainerStart]})
	}
	return chunks
}

func parseUint32(str string) (uint32, error) {
	i, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parseUint32: %v", err)
	}
	return uint32(i), nil
}
