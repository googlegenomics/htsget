package cram

import (
	"bytes"
	"os"
	"testing"
)

func TestGetReferenceID(t *testing.T) {
	testCases := []struct {
		file      string
		reference string
		want      int32
	}{
		{"reference.cram", "chr2", 1},
		{"reference.cram", "chr3", 2},
		{"reference2.cram", "phix-illumina.fa", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.file+":"+tc.reference, func(t *testing.T) {
			f, err := os.Open("testdata/" + tc.file)
			if err != nil {
				t.Fatalf("Failed to open test file: %v", err)
			}

			got, err := GetReferenceID(f, tc.reference)
			if err != nil {
				t.Fatalf("Failed to get reference ID: %v", err)
			}
			if got != tc.want {
				t.Errorf("Incorrect reference ID: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReadITF8(t *testing.T) {
	testCases := []struct {
		name  string
		bytes []byte
		want  int32
	}{
		{"zero", []byte{0}, 0},
		{"one byte max", []byte{0x7f}, 0x7f},
		{"two byte", []byte{0x81, 0x02}, 0x0102},
		{"two byte max", []byte{0xbf, 0xff}, 0x3fff},
		{"three byte", []byte{0xc1, 0x02, 0x03}, 0x010203},
		{"three byte max", []byte{0xdf, 0xff, 0xff}, 0x1fffff},
		{"four byte", []byte{0xe1, 0x02, 0x03, 0x04}, 0x01020304},
		{"four byte max", []byte{0xef, 0xff, 0xff, 0xff}, 0x0fffffff},
		{"five byte", []byte{0xf1, 0x02, 0x03, 0x04, 0x05}, 0x10203045},
		{"five byte max", []byte{0xff, 0xff, 0xff, 0xff, 0x0f}, -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var got int32
			if err := readITF8(bytes.NewReader(tc.bytes), &got); err != nil {
				t.Fatalf("Error reading ITF8 value: %v", tc.bytes, err)
			}
			if got != tc.want {
				t.Errorf("Wrong ITF8 result: got: 0x%08x, want: 0x%08x", uint32(got), uint32(tc.want))
			}
		})
	}
}
