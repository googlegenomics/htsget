// Package genomics contains definitions related to Genomic data.
package genomics

// AllMappedReads defines a Region that matches all mapped reads.
var AllMappedReads = Region{ReferenceID: -1}

// Region defines a region of genomic interest.
type Region struct {
	// ReferenceID specifies the reference to match.  If it is negative, any
	// reference matches the region.
	ReferenceID int32
	// Start and End specify the open range (in base pairs) relative to the
	// reference.  If End is zero, it is treated as though it was set to the last
	// possible read position.
	Start, End uint32
}
