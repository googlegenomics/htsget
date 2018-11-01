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
	"github.com/googlegenomics/htsget/internal/index"
)

const (
	csiMagic = "CSI\x01"
)

// Read reads CSI formatted index data from r and returns a set of BGZF chunks covering the header
// and all mapped reads that fall inside the specified region.  The first chunk is always the BCF
// header.
func Read(r io.Reader, region genomics.Region) ([]*bgzf.Chunk, error) {
	csi, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("initializing gzip reader: %v", err)
	}
	defer csi.Close()
	return index.Read(csi, region, csiMagic, &Reader{})
}

// Reader contains support for reading information from CSI formatted data.
type Reader struct {
}

// ReadSchemeSize reads the CSI formated index data header and returns the scheme size.
func (*Reader) ReadSchemeSize(csi io.Reader) (int32, int32, error) {
	var csiHeader struct {
		MinimumWidth   int32
		Depth          int32
		AuxilaryLength int32
	}
	if err := binary.Read(csi, &csiHeader); err != nil {
		return 0, 0, fmt.Errorf("reading the csi header: %v", err)
	}
	if _, err := io.CopyN(ioutil.Discard, csi, int64(csiHeader.AuxilaryLength)); err != nil {
		return 0, 0, fmt.Errorf("reading past auxiliary data: %v", err)
	}
	return csiHeader.MinimumWidth, csiHeader.Depth, nil
}

// ReadBin reads a bin from r.
func (*Reader) ReadBin(r io.Reader) (*index.Bin, error) {
	var bin index.Bin
	if err := binary.Read(r, &bin); err != nil {
		return nil, fmt.Errorf("reading bin header: %v", err)
	}
	return &bin, nil
}

// IsVirtualBin indicates if the provided ID identifies a virtual bin that is used to store
// metadata.
func (*Reader) IsVirtualBin(uint32) bool {
	return false
}

// SelectChunks appends the candidate chunks to the final list of chunks.
func (*Reader) SelectChunks(_ io.Reader, _ genomics.Region, candidates []*bgzf.Chunk, chunks []*bgzf.Chunk) ([]*bgzf.Chunk, error) {
	for _, chunk := range candidates {
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}
