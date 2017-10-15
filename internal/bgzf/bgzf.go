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

// Package bgzf provides support for parsing BGZF files.
package bgzf

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// LastAddress is the maximum valid BGZF address.
const LastAddress = Address(0xffffffffffffffff)

// MaximumBlockSize is the maximum BGZF block size.
const MaximumBlockSize = 65536

// Address stores a BGZF "virtual address".  The lower 16 bits store the data
// offset inside the uncompressed stream and upper 48 bits store the block
// offset inside the compressed archive set.
type Address uint64

// BlockOffset returns the offset to the start of the compressed block.
func (v Address) BlockOffset() uint64 {
	return uint64(v >> 16)
}

// DataOffset returns the offset to the data in the uncompressed block.
func (v Address) DataOffset() uint16 {
	return uint16(v & 0xffff)
}

// String returns a representation of v that can be parsed with ParseAddress.
func (v Address) String() string {
	return strconv.FormatUint(uint64(v), 16)
}

// ParseAddress attempts to parse input into an Address.
func ParseAddress(input string) (Address, error) {
	v, err := strconv.ParseUint(input, 16, 64)
	return Address(v), err
}

// NewAddress returns a new Address with the provided offsets.
func NewAddress(blockOffset uint64, dataOffset uint16) Address {
	return Address(blockOffset<<16 | uint64(dataOffset))
}

// Chunk specifies a region from Start to End (inclusive) inside a BGZF file.
type Chunk struct {
	Start, End Address
}

// String returns a human readable description of the receiver.
func (v *Chunk) String() string {
	return fmt.Sprintf("[%s-%s]", v.Start, v.End)
}

// Merge attempts to merge any intersecting chunks in input.  Merge will not
// join two chunks if their combined size could exceed sizeLimit.
func Merge(input []*Chunk, sizeLimit uint64) []*Chunk {
	sort.Slice(input, func(i, j int) bool {
		return input[i].Start < input[j].Start
	})

	var (
		merged = []*Chunk{input[0]}
		output = merged[0]
	)
	for i := 1; i < len(input); i++ {
		var size uint64
		if input[i].End.BlockOffset() == output.Start.BlockOffset() {
			size = uint64(input[i].End.DataOffset() - output.Start.DataOffset())
		} else {
			// Estimate using the maximum size for the last block.
			size = input[i].End.BlockOffset() - output.Start.BlockOffset() + MaximumBlockSize
		}

		if input[i].Start <= output.End && size <= sizeLimit {
			if output.End < input[i].End {
				output.End = input[i].End
			}
		} else {
			merged = append(merged, input[i])
			output = merged[len(merged)-1]
		}
	}
	return merged
}

// DecodeBlock decodes a single BGZF block from r and returns the uncompressed
// data and the original block size (or an error).  Note that DecodeBlock may
// read bytes past the end of the block if r does not implement io.ByteReader.
func DecodeBlock(r io.Reader) ([]byte, uint16, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, 0, fmt.Errorf("initializing gzip reader: %v", err)
	}
	defer gzr.Close()

	extra := gzr.Header.Extra
	if extra[0] != 0x42 || extra[1] != 0x43 {
		return nil, 0, fmt.Errorf("unexpected extra ID: %x", extra[0:2])
	}
	if extra[2] != 2 || extra[3] != 0 {
		return nil, 0, fmt.Errorf("unexpected extra length: %x", extra[2:4])
	}

	gzr.Multistream(false)
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, gzr); err != nil {
		return nil, 0, fmt.Errorf("decompressing data: %v", err)
	}
	return buffer.Bytes(), (uint16(extra[4]) | uint16(extra[5])<<8) + 1, nil
}

// EncodeBlock returns a single BGZF block that encodes the bytes in data.
func EncodeBlock(data []byte) ([]byte, error) {
	if len(data) > MaximumBlockSize {
		return nil, errors.New("data exceeds maximum block size")
	}

	var buffer bytes.Buffer
	gzw := gzip.NewWriter(&buffer)

	gzw.Header.Extra = []byte{
		0x42, 0x43, // Extra ID.
		0x02, 0x00, // Length of extra data (2 bytes).
		0x88, 0x88, // BSIZE (filled in after writing the archive).
	}
	if _, err := gzw.Write(data); err != nil {
		return nil, fmt.Errorf("writing compressed data: %v", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("closing writer: %v", err)
	}
	bsize := buffer.Len() - 1
	encoded := buffer.Bytes()
	encoded[16] = byte(bsize)
	encoded[17] = byte(bsize >> 8)
	return encoded, nil
}
