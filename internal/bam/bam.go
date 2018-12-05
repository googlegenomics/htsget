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
	"github.com/googlegenomics/htsget/internal/index"
)

const (
	baiMagic = "BAI\x01"
	bamMagic = "BAM\x01"

	// This ID is used as a virtual bin ID for (unused) chunk metadata.
	metadataID = 37450

	// This is just to prevent arbitrarily long allocations due to malformed
	// data.  No reference name should be longer than this in practice.
	maximumNameLength = 1024

	// The size of each tiling window from the linear index, as specified in the
	// SAM specification section 5.1.3.
	linearWindowSize = 1 << 14
)

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
	return index.Read(bai, region, baiMagic, &BAIReader{})
}

// BAIReader contains support for reading information from BAI formatted data.
type BAIReader struct {
}

// ReadSchemeSize returns the scheme size.  BAM uses a 6 level (depth = 5) CSI binning scheme with
// a minimum width of 14 bits.
func (*BAIReader) ReadSchemeSize(_ io.Reader) (int32, int32, error) {
	return 14, 5, nil
}

// ReadBin reads a bin from r.
func (*BAIReader) ReadBin(r io.Reader) (*index.Bin, error) {
	var bin struct {
		ID     uint32
		Chunks int32
	}
	if err := binary.Read(r, &bin); err != nil {
		return nil, fmt.Errorf("reading bin header: %v", err)
	}

	return &index.Bin{
		ID:     bin.ID,
		Chunks: bin.Chunks,
	}, nil
}

// IsVirtualBin indicates if the provided ID identifies a virtual bin that is used to store
// metadata.
func (*BAIReader) IsVirtualBin(ID uint32) bool {
	return ID == metadataID
}

// SelectChunks reads the list of intervals from the bai reader, filters the candidate chunks that
// overlap the requested region and append them to the final list of chunks.
func (*BAIReader) SelectChunks(bai io.Reader, region genomics.Region, candidates []*bgzf.Chunk, chunks []*bgzf.Chunk) ([]*bgzf.Chunk, error) {
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
	return chunks, nil
}
