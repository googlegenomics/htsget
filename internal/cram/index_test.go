package cram

import (
	"bytes"
	"compress/gzip"
	"math"
	"reflect"
	"testing"

	"github.com/googlegenomics/htsget/internal/genomics"
)

func TestReadIndex(t *testing.T) {
	buffer := compress(`1 2 3 4 5 6
7 8 9 10 11 12`)
	want := &Index{
		[]indexEntry{
			{1, 2, 3, 4},
			{7, 8, 9, 10},
		},
		map[uint64]uint64{
			0:  4,
			4:  10,
			10: math.MaxUint64,
		},
	}

	got, err := ReadIndex(buffer)
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("incorrect index, got: %v, want: %v", got, want)
	}
}

func TestGetChunksForRegion(t *testing.T) {
	index, err := ReadIndex(compress(`1 1 100 1000 0 0
1 50 100 2000 0 0
2 1 150 3000 0 0`))
	if err != nil {
		t.Fatalf("reading index: %v", err)
	}

	testCases := []struct {
		name   string
		region genomics.Region
		want   []*Chunk
	}{
		{
			"empty reference",
			genomics.Region{3, 0, 0},
			[]*Chunk{{0, 1000}},
		},
		{
			"reference 1",
			genomics.Region{1, 0, 0},
			[]*Chunk{{0, 1000}, {1000, 2000}, {2000, 3000}},
		},
		{
			"reference 2",
			genomics.Region{2, 0, 0},
			[]*Chunk{{0, 1000}, {3000, math.MaxUint64}},
		},
		{
			"all reads",
			genomics.AllMappedReads,
			[]*Chunk{{0, 1000}, {1000, 2000}, {2000, 3000}, {3000, math.MaxUint64}},
		},
		{
			"disjoint range",
			genomics.Region{-1, 10, 20},
			[]*Chunk{{0, 1000}, {1000, 2000}, {3000, math.MaxUint64}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := index.GetChunksForRegion(tc.region)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("incorrect chunks, got: %v, want: %v", got, tc.want)
			}
		})
	}
}

func compress(index string) *bytes.Buffer {
	var buffer bytes.Buffer
	w := gzip.NewWriter(&buffer)
	w.Write([]byte(index))
	w.Close()
	return &buffer
}
