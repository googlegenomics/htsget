package cram

import (
	"reflect"
	"testing"
)

func TestSortAndMerge(t *testing.T) {
	testCases := []struct {
		name      string
		chunks    []*Chunk
		sizeLimit uint64
		want      []*Chunk
	}{
		{
			"unordered, unlimited chunks",
			[]*Chunk{{10, 20}, {20, 30}, {0, 5}, {5, 9}},
			20,
			[]*Chunk{{0, 9}, {10, 30}},
		},
		{
			"unordered, limited chunks",
			[]*Chunk{{10, 20}, {20, 30}, {0, 5}, {5, 9}},
			10,
			[]*Chunk{{0, 9}, {10, 20}, {20, 30}},
		},
		{
			"ordered, unlimited chunks",
			[]*Chunk{{0, 5}, {5, 10}, {10, 20}},
			20,
			[]*Chunk{{0, 20}},
		},
		{
			"ordered, limited chunks",
			[]*Chunk{{0, 5}, {5, 10}, {10, 20}},
			1,
			[]*Chunk{{0, 5}, {5, 10}, {10, 20}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SortAndMerge(tc.chunks, tc.sizeLimit)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SortAndMerge(sizeLimit=%d): got %v, want %v", tc.sizeLimit, got, tc.want)
			}
		})
	}
}
