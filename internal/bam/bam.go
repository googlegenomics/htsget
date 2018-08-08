// Copyright 2017 Google Inc.
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

// Package bam provides support for parsing BAM files.
package bam

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
	baiMagic = "BAI\x01"
	bamMagic = "BAM\x01"

	// This ID is used as a virtual bin ID for (unused) chunk metadata.
	metadataID = 37450

	// This is just to prevent arbitrarily long allocations due to malformed
	// data.  No reference name should be longer than this in practice.
	maximumNameLength = 1024

	// The maximum read length as constrained by the size of the level zero bin
	// in the SAM specification, section 5.1.1.
	maximumReadLength = 1 << 29

	// The size of each tiling window from the linear index, as specified in the
	// SAM specification section 5.1.3.
	linearWindowSize = 1 << 14
)

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

// GetReferenceID attempts to determine the ID for the named genomic reference
// by reading BAM header data from bam.
func GetReferenceID(bam io.Reader, reference string) (int32, error) {
	bam, err := gzip.NewReader(bam)
	if err != nil {
		return 0, fmt.Errorf("opening archive: %v", err)
	}

	if err := binary.ExpectBytes(bam, []byte(bamMagic)); err != nil {
		return 0, fmt.Errorf("reading magic: %v", err)
	}
	var length int32
	if err := binary.Read(bam, &length); err != nil {
		return 0, fmt.Errorf("reading SAM header length: %v", err)
	}
	if _, err := io.CopyN(ioutil.Discard, bam, int64(length)); err != nil {
		return 0, fmt.Errorf("reading past SAM header: %v", err)
	}
	var count int32
	if err := binary.Read(bam, &count); err != nil {
		return 0, fmt.Errorf("reading references count: %v", err)
	}
	for i := int32(0); i < count; i++ {
		if err := binary.Read(bam, &length); err != nil {
			return 0, fmt.Errorf("reading name length: %v", err)
		}
		// The name length includes a null terminating character.
		if length < 1 || length > maximumNameLength {
			return 0, fmt.Errorf("invalid name length (%d bytes)", length)
		}
		name := make([]byte, length)
		if _, err := bam.Read(name); err != nil {
			return 0, fmt.Errorf("reading name: %v", err)
		}
		if string(name[:length-1]) == reference {
			return i, nil
		}
		// Read and discard the reference length (4 bytes);
		if err := binary.Read(bam, &length); err != nil {
			return 0, fmt.Errorf("reading reference length: %v", err)
		}
	}
	return 0, fmt.Errorf("no reference named %q found", reference)
}

// Read reads index data from bai and returns a set of BGZF chunks covering
// the header and all mapped reads that fall inside the specified region.  The
// first chunk is always the BAM header.
func Read(bai io.Reader, region genomics.Region) ([]*bgzf.Chunk, error) {
	if err := binary.ExpectBytes(bai, []byte(baiMagic)); err != nil {
		return nil, fmt.Errorf("reading magic: %v", err)
	}

	var references int32
	if err := binary.Read(bai, &references); err != nil {
		return nil, fmt.Errorf("reading reference count: %v", err)
	}

	bins := binsForRange(region.Start, region.End)

	header := &bgzf.Chunk{End: bgzf.LastAddress}
	chunks := []*bgzf.Chunk{header}
	for i := int32(0); i < references; i++ {
		var binCount int32
		if err := binary.Read(bai, &binCount); err != nil {
			return nil, fmt.Errorf("reading bin count: %v", err)
		}
		var candidates []*bgzf.Chunk
		for j := int32(0); j < binCount; j++ {
			var bin struct {
				ID     uint32
				Chunks int32
			}
			if err := binary.Read(bai, &bin); err != nil {
				return nil, fmt.Errorf("reading bin header: %v", err)
			}

			includeChunks := regionContainsBin(region, i, bin.ID, bins)
			for k := int32(0); k < bin.Chunks; k++ {
				var chunk bgzf.Chunk
				if err := binary.Read(bai, &chunk); err != nil {
					return nil, fmt.Errorf("reading chunk: %v", err)
				}
				if bin.ID == metadataID {
					continue
				}
				if includeChunks {
					candidates = append(candidates, &chunk)
				}
				if header.End > chunk.Start {
					header.End = chunk.Start
				}
			}
		}

		var intervals int32
		if err := binary.Read(bai, &intervals); err != nil {
			return nil, fmt.Errorf("reading interval count: %v", err)
		}
		if intervals < 0 {
			return nil, fmt.Errorf("invalid interval count (%d intervals)", intervals)
		}
		offsets := make([]uint64, intervals)
		if err := binary.Read(bai, &offsets); err != nil {
			return nil, fmt.Errorf("reading offsets: %v", err)
		}

		var firstReadOffset bgzf.Address
		if index := int(region.Start / linearWindowSize); index < len(offsets) {
			firstReadOffset = bgzf.Address(offsets[index])
		}

		for _, chunk := range candidates {
			if chunk.End < firstReadOffset {
				continue
			}
			chunks = append(chunks, chunk)
		}
	}
	return chunks, nil
}

// This function is derived from the C examples in the BAM index specification.
func binsForRange(start, end uint32) []uint16 {
	if end == 0 || end > maximumReadLength {
		end = maximumReadLength
	}
	if end <= start {
		return nil
	}
	if start > maximumReadLength {
		return nil
	}

	end--

	bins := []uint16{0}
	for k := uint16(1 + (start >> 26)); k <= uint16(1+(end>>26)); k++ {
		bins = append(bins, k)
	}
	for k := uint16(9 + (start >> 23)); k <= uint16(9+(end>>23)); k++ {
		bins = append(bins, k)
	}
	for k := uint16(73 + (start >> 20)); k <= uint16(73+(end>>20)); k++ {
		bins = append(bins, k)
	}
	for k := uint16(585 + (start >> 17)); k <= uint16(585+(end>>17)); k++ {
		bins = append(bins, k)
	}
	for k := uint16(4681 + (start >> 14)); k <= uint16(4681+(end>>14)); k++ {
		bins = append(bins, k)
	}
	return bins
}
