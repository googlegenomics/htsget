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
	"github.com/googlegenomics/htsget/internal/genomics"
)

// RegionContainsBin indicates if the given region contains the bin described by
// referenceID and binID.
func RegionContainsBin(region genomics.Region, referenceID int32, binID uint32, bins []uint16) bool {
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

// BinsForRange returns the list of bins that may overlap with the zero-based region
// defined by [start, end). The minShift and depth parameters control the minimum interval width
// and number of binning levels, respectively.
func BinsForRange(start, end uint32, minShift, depth int32) []uint16 {
	maxWidth := maximumBinWidth(minShift, depth)
	if end == 0 || end > maxWidth {
		end = maxWidth
	}
	if end <= start {
		return nil
	}
	if start > maxWidth {
		return nil
	}

	// This is derived from the C examples in the CSI index specification.
	end--
	var bins []uint16
	for l, t, s := uint(0), uint(0), uint(minShift+depth*3); l <= uint(depth); l++ {
		b := t + (uint(start) >> s)
		e := t + (uint(end) >> s)
		for i := b; i <= e; i++ {
			bins = append(bins, uint16(i))
		}
		s -= 3
		t += 1 << (l * 3)
	}
	return bins
}

func maximumBinWidth(minShift, depth int32) uint32 {
	return uint32(1 << uint32(minShift+depth*3))
}
