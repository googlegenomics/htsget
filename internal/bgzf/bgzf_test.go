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

package bgzf

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

func TestAddress(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		block uint64
		data  uint16
	}{
		{"maximum value", "ffffffffffffffff", 0x0000ffffffffffff, 0xffff},
		{"zero data offset", "ffff0000", 0xffff, 0x0000},
		{"zero", "0", 0, 0},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			address, err := ParseAddress(tc.input)
			if err != nil {
				t.Fatalf("Got error parsing %q: %v", tc.input, err)
			}
			if got, want := address.BlockOffset(), tc.block; got != want {
				t.Errorf("Wrong block offset: got 0x%016x, want 0x%016x", got, want)
			}
			if got, want := address.DataOffset(), tc.data; got != want {
				t.Errorf("Wrong data offset: got 0x%04x, want 0x%04x", got, want)
			}
			if got, want := address.String(), tc.input; got != want {
				t.Errorf("Wrong string result: got %q, want %q", got, want)
			}
		})
	}
}

func TestParseAddress_InvalidInputs(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"negative value", "-0"},
		{"too large", "ffffffffffffffffffff"},
		{"non-hexidecimal", "g"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ParseAddress(tc.input); err == nil {
				t.Errorf("Unexpected success: got %v, wanted error", got)
			}
		})
	}
}

func TestChunk_String(t *testing.T) {
	testCases := []struct {
		name       string
		start, end Address
		want       string
	}{
		{"zero", 0, 0, "[0-0]"},
		{"same block", 0, 0xffff, "[0-ffff]"},
		{"different block", 0, 0xaffff, "[0-affff]"},
		{"0 -> limit", 0, LastAddress, "[0-ffffffffffffffff]"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunk := &Chunk{tc.start, tc.end}
			if got, want := chunk.String(), tc.want; got != want {
				t.Errorf("String(): got %q, want %q", got, want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	testCases := []struct {
		name   string
		limit  uint64
		input  string
		merged string
	}{
		{
			"three chunks, same block, all overlapping",
			1024,
			"0-10,10-40,40-80",
			"0-80",
		},
		{
			"three chunks, same block, one not overlapping",
			1024,
			"0-10,20-40,40-80",
			"0-10,20-80",
		},
		{
			"unsorted (but mergeable) chunks",
			1024,
			"40-80,10-40,0-10",
			"0-80",
		},
		{
			"two chunks, same block, too large",
			32768,
			"0-8000,9000-a000",
			"0-8000,9000-a000",
		},
		{
			"two chunks, same block, exactly small enough",
			32768,
			"0-7000,7000-8000",
			"0-8000",
		},
		{
			"two chunks, different blocks, ok to merge",
			64*1024 + 4096,
			"00000000-00008000,00008000-10000000",
			"00000000-10000000",
		},
		{
			"two chunks, different blocks, too big",
			64*1024 + 4096 - 1,
			"00000000-00008000,00008000-10000000",
			"00000000-00008000,00008000-10000000",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input, err := parseChunkString(tc.input)
			if err != nil {
				t.Fatalf("Bad chunk string: %v", err)
			}
			want, err := parseChunkString(tc.merged)
			if got := Merge(input, tc.limit); !reflect.DeepEqual(got, want) {
				t.Errorf("Merge: got %s, want %s", got, want)
			}
		})
	}
}

func TestDecodeBlock(t *testing.T) {
	// Read test data to memory and use a ByteReader so that the gzip reader
	// doesn't read too many bytes (it does if the reader only implements Read).
	input, err := ioutil.ReadFile("testdata/tiny.bam")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}
	r := bytes.NewReader(input)

	blocks := []struct {
		bsize uint16
		isize uint16
	}{
		{223, 296}, /* Header */
		{420, 827}, /* Data */
		{28, 0},    /* EOF marker */
	}
	for i, block := range blocks {
		data, length, err := DecodeBlock(r)
		if err != nil {
			t.Fatalf("Failed to read block %d: %v", i, err)
		}

		if got, want := length, block.bsize; got != want {
			t.Errorf("Wrong compressed block length: got %d, want %d", got, want)
		}

		if got, want := uint16(len(data)), block.isize; got != want {
			t.Errorf("Wrong uncompressed data length: got %d, want %d", got, want)
		}
	}
}

func TestEncodeBlock_ValidInputs(t *testing.T) {
	testCases := []struct {
		name       string
		data, want []byte
	}{
		{"empty block (EOF marker, embedded zlib sync marker)", nil, []byte{
			0x1f, 0x8b, 0x08, 0x04, 0x00, 0x00, 0x00, 0x00,
			0x00, 0xff, 0x06, 0x00, 0x42, 0x43, 0x02, 0x00,
			0x1e, 0x00, 0x01, 0x00, 0x00, 0xff, 0xff, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		}},
		{"single byte block", []byte{0x42}, []byte{
			0x1f, 0x8b, 0x08, 0x04, 0x00, 0x00, 0x00, 0x00,
			0x00, 0xff, 0x06, 0x00, 0x42, 0x43, 0x02, 0x00,
			0x20, 0x00, 0x72, 0x02, 0x04, 0x00, 0x00, 0xff,
			0xff, 0x31, 0xcf, 0xd0, 0x4a, 0x01, 0x00, 0x00,
			0x00,
		}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeBlock(tc.data)
			if err != nil {
				t.Fatalf("Failed to write block: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("WriteBlock(): got %x, want %x", got, tc.want)
			}
		})
	}
}

func TestEncodeBlock_BlockSizes(t *testing.T) {
	if _, err := EncodeBlock(make([]byte, MaximumBlockSize+1)); err == nil {
		t.Fatal("EncodeBlock() should fail with block over size limit but didn't")
	}
	if _, err := EncodeBlock(make([]byte, MaximumBlockSize)); err != nil {
		t.Fatal("EncodeBlock() should succeed with block at size limit but didn't")
	}
}

func parseChunkString(input string) ([]*Chunk, error) {
	var chunks []*Chunk
	for _, s := range strings.Split(input, ",") {
		v := strings.Split(s, "-")
		start, err := ParseAddress(v[0])
		if err != nil {
			return nil, fmt.Errorf("parsing chunk start: %v", err)
		}
		end, err := ParseAddress(v[1])
		if err != nil {
			return nil, fmt.Errorf("parsing chunk end: %v", err)
		}
		chunks = append(chunks, &Chunk{start, end})
	}
	return chunks, nil
}
