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

// Package csi contains support for processing the information in a CSI file (http://samtools.github.io/hts-specs/CSIv1.pdf).
package csi

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/binary"
	"github.com/googlegenomics/htsget/internal/genomics"
)

const (
	csiMagic = "CSI\x01"
)

// RegionContainsBin indicates if the given region contains the bin described by
// referenceID and binID.
func RegionContainsBin(region genomics.Region, referenceID int32, binID uint32, bins []uint16) bool {
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

// BinsForRange returns the list of bins that may overlap with the zero-based region
// defined by [start, end). The minShift and depth parameters control the minimum interval width
// and number of binning levels, respectively.
func BinsForRange(start, end uint32, minShift, depth int32) []uint16 {
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

// Read reads index data from csi and returns a set of BGZF chunks covering
// the header and all mapped reads that fall inside the specified region.  The
// first chunk is always the BCF header.
func Read(csiFile io.Reader, region genomics.Region) ([]*bgzf.Chunk, error) {
	gzr, err := gzip.NewReader(csiFile)
	if err != nil {
		return nil, fmt.Errorf("initializing gzip reader: %v", err)
	}
	defer gzr.Close()
	if err := binary.ExpectBytes(gzr, []byte(csiMagic)); err != nil {
		return nil, fmt.Errorf("checking magic: %v", err)
	}

	var minShift int32
	if err := binary.Read(gzr, &minShift); err != nil {
		return nil, fmt.Errorf("reading # bits for the minimal interval (min_shift): %v", err)
	}
	var depth int32
	if err := binary.Read(gzr, &depth); err != nil {
		return nil, fmt.Errorf("reading depth of binary index: %v", err)
	}
	bins := BinsForRange(region.Start, region.End, minShift, depth)

	var laux int32
	if err := binary.Read(gzr, &laux); err != nil {
		return nil, fmt.Errorf("reading length of auxiliary data: %v", err)
	}
	if _, err := io.CopyN(ioutil.Discard, gzr, int64(laux)); err != nil {
		return nil, fmt.Errorf("reading past auxiliary data: %v", err)
	}

	header := &bgzf.Chunk{End: bgzf.LastAddress}
	chunks := []*bgzf.Chunk{header}
	var refCount int32
	if err := binary.Read(gzr, &refCount); err != nil {
		return nil, fmt.Errorf("reading the number of reference sequences: %v", err)
	}
	for reference := int32(0); reference < refCount; reference++ {
		var binCount int32
		if err := binary.Read(gzr, &binCount); err != nil {
			return nil, fmt.Errorf("reading bin count: %v", err)
		}
		for j := int32(0); j < binCount; j++ {
			var bin struct {
				ID     uint32
				Offset uint64
				Chunks int32
			}
			if err := binary.Read(gzr, &bin); err != nil {
				return nil, fmt.Errorf("reading bin header: %v", err)
			}

			includeChunks := RegionContainsBin(region, reference, bin.ID, bins)
			for k := int32(0); k < bin.Chunks; k++ {
				var chunk bgzf.Chunk
				if err := binary.Read(gzr, &chunk); err != nil {
					return nil, fmt.Errorf("reading chunk: %v", err)
				}
				if includeChunks && (chunk.End >= bgzf.Address(bin.Offset)) {
					chunks = append(chunks, &chunk)
				}
				if header.End > chunk.Start {
					header.End = chunk.Start
				}
			}
		}
	}
	return chunks, nil
}
