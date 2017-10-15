package sam

import (
	"fmt"
	"os"
	"testing"
)

func TestGetReferenceID(t *testing.T) {
	testCases := []struct {
		file string
		refs map[string]int32
	}{
		{
			"simple.header",
			map[string]int32{
				"r0":   0,
				"r0a0": 0,
				"r1":   1,
				"r1a0": 1,
				"r1a1": 1,
				"r2":   2,
			},
		},
		{
			"complex.header",
			map[string]int32{
				"1":          0,
				"2":          1,
				"testA":      1,
				"testB":      1,
				"5":          2,
				"GL000226.1": 3,
				"GL000229.1": 4,
			},
		},
	}

	for _, tc := range testCases {
		for ref, want := range tc.refs {
			t.Run(fmt.Sprintf("%s-%s", tc.file, ref), func(t *testing.T) {
				f, err := os.Open("testdata/" + tc.file)
				if err != nil {
					t.Fatalf("Error reading test file: %v", err)
				}
				defer f.Close()

				if got, err := GetReferenceID(f, ref); err != nil {
					t.Errorf("Error getting reference ID: %v", err)
				} else if got != want {
					t.Errorf("Incorrect ID: got %d, want %d", got, want)
				}
			})
		}
	}
}
