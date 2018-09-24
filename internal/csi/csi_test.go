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

package csi

import (
	"testing"

	"math"
	"reflect"
)

func TestBinsForRange(t *testing.T) {
	metadataID := 37450
	allBins := make([]uint16, metadataID-1)
	for i := range allBins {
		allBins[i] = uint16(i)
	}

	testCases := []struct {
		name            string
		start, end      uint32
		minShift, depth int32
		bins            []uint16
	}{
		{"end clamping", 0, math.MaxUint32, 14, 5, allBins},
		{"end past maximum", 0, maximumReadLength(14, 5) + 1, 14, 5, allBins},
		{"start past maximum", maximumReadLength(14, 5) + 1, maximumReadLength(14, 5) + 2, 14, 5, nil},
		{"narrow region", 0, 1, 14, 5, []uint16{0, 1, 9, 73, 585, 4681}},
		{"narrow depth", 0, 1, 14, 4, []uint16{0, 1, 9, 73, 585}},
		{"invalid range (start > end)", math.MaxUint32, 0, 14, 5, nil},
		{"swapped endpoints", 2, 1, 14, 5, nil},
		{"zero-width region", 1, 1, 14, 5, nil},
		{"zero end", 1, 0, 14, 5, allBins},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := BinsForRange(tc.start, tc.end, tc.minShift, tc.depth), tc.bins; !reflect.DeepEqual(got, want) {
				t.Fatalf("BinsForRange(%v, %v) = %+v, want %+v", tc.start, tc.end, got, want)
			}
		})
	}
}
