// Copyright 2018 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package index contains support for processing the information in a index file.
package index

import (
	"fmt"
	"io"

	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/binary"
	"github.com/googlegenomics/htsget/internal/genomics"
)

// Read reads index data from r and returns a set of BGZF chunks covering the header and all mapped
// reads that fall inside the specified region.  The first chunk is always the header of the indexed
// file.  The function takes a reader that reads format specific information from the input reader.
func Read(r io.Reader, region genomics.Region, magic string, reader Reader) ([]*bgzf.Chunk, error) {
	if err := binary.ExpectBytes(r, []byte(magic)); err != nil {
		return nil, fmt.Errorf("reading magic: %v", err)
	}

	width, depth, err := reader.ReadSchemeSize(r)
	if err != nil {
		return nil, fmt.Errorf("reading the scheme size: %v", err)
	}
	bins := binsForRange(region.Start, region.End, width, depth)

	var references int32
	if err := binary.Read(r, &references); err != nil {
		return nil, fmt.Errorf("reading reference count: %v", err)
	}

	header := &bgzf.Chunk{End: bgzf.LastAddress}
	chunks := []*bgzf.Chunk{header}
	for i := int32(0); i < references; i++ {
		var binCount int32
		if err := binary.Read(r, &binCount); err != nil {
			return nil, fmt.Errorf("reading bin count: %v", err)
		}

		var candidates []*bgzf.Chunk
		for j := int32(0); j < binCount; j++ {
			bin, err := reader.ReadBin(r)
			if err != nil {
				return nil, fmt.Errorf("reading bin: %v", err)
			}

			includeChunks := regionContainsBin(region, i, bin.ID, bins)
			for k := int32(0); k < bin.Chunks; k++ {
				var chunk bgzf.Chunk
				if err := binary.Read(r, &chunk); err != nil {
					return nil, fmt.Errorf("reading chunk: %v", err)
				}
				if reader.IsVirtualBin(bin.ID) {
					continue
				}
				if includeChunks && (chunk.End >= bgzf.Address(bin.Offset)) {
					candidates = append(candidates, &chunk)
				}
				if header.End > chunk.Start {
					header.End = chunk.Start
				}
			}
		}
		chunks, err = reader.SelectChunks(r, region, candidates, chunks)
		if err != nil {
			return nil, fmt.Errorf("selecting chunks: %v", err)
		}
	}
	return chunks, nil
}

// Reader is an interface for reading format specific information from index data.
type Reader interface {
	// ReadSchemeSize reads the binning scheme's width which is the number of bits for
	// the minimal interval and the depth of the binning index.
	ReadSchemeSize(io.Reader) (int32, int32, error)
	// ReadBin reads a bin.
	ReadBin(io.Reader) (*Bin, error)
	// IsVirtualBin indicates if the provided ID identifies a virtual bin that is used to store
	// metadata.
	IsVirtualBin(uint32) bool
	// SelectChunks filters the candidate chunks that overlap the requested region and append them to
	// the final list of chunks.
	SelectChunks(io.Reader, genomics.Region, []*bgzf.Chunk, []*bgzf.Chunk) ([]*bgzf.Chunk, error)
}

// Bin represents a contignous genomic region.
type Bin struct {
	// ID is an identifier for the bin.
	ID uint32
	// Offset is the (virtual) file offset of the first overlapping record.
	Offset uint64
	// Chunks is the number of chunks in the bin.
	Chunks int32
}

func regionContainsBin(region genomics.Region, referenceID int32, binID uint32, bins []uint16) bool {
	if region.ReferenceID >= 0 && referenceID != region.ReferenceID {
		return false
	}

	if region.Start == 0 && region.End == 0 {
		return true
	}

	for _, id := range bins {
		if uint32(id) == binID {
			return true
		}
	}
	return false
}

func binsForRange(start, end uint32, minShift, depth int32) []uint16 {
	maxWidth := maximumBinWidth(minShift, depth)
	if end == 0 || end > maxWidth {
		end = maxWidth
	}
	if end <= start {
		return nil
	}
	if start > maxWidth {
		return nil
	}

	// This is derived from the C examples in the CSI index specification.
	end--
	var bins []uint16
	for l, t, s := uint(0), uint(0), uint(minShift+depth*3); l <= uint(depth); l++ {
		b := t + (uint(start) >> s)
		e := t + (uint(end) >> s)
		for i := b; i <= e; i++ {
			bins = append(bins, uint16(i))
		}
		s -= 3
		t += 1 << (l * 3)
	}
	return bins
}

func maximumBinWidth(minShift, depth int32) uint32 {
	return uint32(1 << uint32(minShift+depth*3))
}
