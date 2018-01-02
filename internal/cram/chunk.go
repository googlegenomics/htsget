package cram

import (
	"fmt"
	"sort"
)

// Chunk specifies a region from Start to End (exclusive) inside a CRAM file.
type Chunk struct {
	Start, End uint64
}

// Length returns the length of a Chunk.
func (c *Chunk) Length() uint64 {
	return c.End - c.Start
}

// String returns a human readable description of the receiver.
func (c *Chunk) String() string {
	return fmt.Sprintf("[%d-%d]", c.Start, c.End)
}

// SortAndMerge sorts the chunks by start position and merges adjacent chunks
// as long as the resultant chunk will not exceed sizeLimit.  The chunks must
// not overlap.
func SortAndMerge(chunks []*Chunk, sizeLimit uint64) []*Chunk {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Start < chunks[j].Start
	})

	var merged []*Chunk
	var last *Chunk
	for _, r := range chunks {
		if last == nil || last.End != r.Start || last.Length()+r.Length() > sizeLimit {
			merged = append(merged, r)
			last = r
		} else {
			last.End = r.End
		}
	}

	return merged
}
