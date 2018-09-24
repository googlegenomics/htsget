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

package bam

import (
	"bytes"
	"os"
	"testing"

	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/genomics"
)

func TestGetReferenceID_Success(t *testing.T) {
	testCases := []struct {
		name string
		id   int32
	}{
		{"1", 0},
		{"20", 19},
		{"GL000249.1", 38},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := os.Open("testdata/multi-reference.bam")
			if err != nil {
				t.Fatalf("Failed to open testdata: %v", err)
			}
			defer r.Close()

			if id, err := GetReferenceID(r, tc.name); err != nil {
				t.Fatalf("GetReferenceID() returned error: %v", err)
			} else if id != tc.id {
				t.Fatalf("Wrong reference ID: got %d, want %d", id, tc.id)
			}
		})
	}
}

func TestGetReferenceID_Errors(t *testing.T) {
	testCases := []struct {
		name      string
		reference string
		data      []byte
	}{
		{"zero-length", "", nil},
		{"wrong magic", "T", []byte{
			'B', 'A', 'M', 2,
			0, 0, 0, 0,
			1, 0, 0, 0,
			1, 0, 0, 0,
			'T', 0,
			0, 0, 0, 0,
		}},
		{"truncated before header length", "", []byte{'B', 'A', 'M', 1}},
		{"truncated header", "", []byte{'B', 'A', 'M', 1, 1, 0, 0, 0}},
		{"truncated before reference count", "",
			[]byte{'B', 'A', 'M', 1, 0, 0, 0, 0},
		},
		{"invalid name length", "X", []byte{
			'B', 'A', 'M', 1,
			0, 0, 0, 0,
			1, 0, 0, 0,
			0, 0, 1, 0,
			'A', 0,
			0, 0, 0, 0,
		}},
		{"truncated name", "X", []byte{
			'B', 'A', 'M', 1,
			0, 0, 0, 0,
			1, 0, 0, 0,
			2, 0, 0, 0,
			'A',
		}},
		{"truncated reference list", "X", []byte{
			'B', 'A', 'M', 1,
			0, 0, 0, 0,
			2, 0, 0, 0,
			1, 0, 0, 0,
			'A',
			0, 0, 0, 0,
		}},
		{"missing reference", "X", []byte{
			'B', 'A', 'M', 1,
			0, 0, 0, 0,
			1, 0, 0, 0,
			1, 0, 0, 0,
			'A', 0,
			0, 0, 0, 0,
		}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			block, err := bgzf.EncodeBlock(tc.data)
			if err != nil {
				t.Fatalf("EncodeBlock() failed: %v", err)
			}

			r := bytes.NewReader(block)
			if _, err := GetReferenceID(r, tc.reference); err == nil {
				t.Fatalf("GetReferenceID(): expected error, not success")
			} else {

				t.Logf("error: %v", err)
			}
		})
	}
}

func TestRead_ChunkCountAndHeaderSize(t *testing.T) {
	testCases := []struct {
		filename   string
		chunks     int
		dataOffset bgzf.Address
	}{
		{"header-in-separate-chunk.bam.bai", 2, 0x12490000},
		{"header-shares-data-chunk.bam.bai", 230, 0x0000532e},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			r, err := os.Open("testdata/" + tc.filename)
			if err != nil {
				t.Fatalf("Failed to open test data: %v", err)
			}

			chunks, err := Read(r, genomics.AllMappedReads)
			if err != nil {
				t.Fatalf("Failed to read test data: %v", err)
			}

			if got, want := len(chunks), tc.chunks; got != want {
				t.Errorf("Wrong number of chunks: got %d, want %d", got, want)
			}
			if got, want := chunks[0].End, tc.dataOffset; got != want {
				t.Errorf("Wrong end address for header: got %s, want %s", got, want)
			}
		})
	}
}

func TestRead_Region(t *testing.T) {
	testCases := []struct {
		name   string
		region genomics.Region
		chunks int
	}{
		{"all mapped reads", genomics.AllMappedReads, 230},
		{"chromosome 19, all reads", genomics.Region{ReferenceID: 18}, 1},
		{"chromosome 20, all reads", genomics.Region{ReferenceID: 19}, 230},
		{"chromosome 20, some reads", genomics.Region{
			ReferenceID: 19,
			Start:       62500000,
			End:         63500000,
		}, 6},
		{"chromosome 20, zero end", genomics.Region{
			ReferenceID: 19,
			Start:       12500000,
		}, 173},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := os.Open("testdata/multi-reference.bam.bai")
			if err != nil {
				t.Fatalf("Failed to open test data: %v", err)
			}
			defer r.Close()

			chunks, err := Read(r, tc.region)
			if err != nil {
				t.Fatalf("Failed to read test data: %v", err)
			}
			if got, want := len(chunks), tc.chunks; got != want {
				t.Fatalf("Wrong number of chunks: got %d, want %d", got, want)
			}
		})
	}
}
