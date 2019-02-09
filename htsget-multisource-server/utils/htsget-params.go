package utils

import (
	"fmt"
	"strconv"

	"github.com/googlegenomics/htsget/internal/bgzf"
)

func HTSGETParams(params map[string]string) (bgzf.Chunk, string, error) {

	chunk := bgzf.Chunk{}
	id := params["id"]
	if id == "" {
		return chunk, "", fmt.Errorf("invalid ID")
	}
	start := params["start"]
	end := params["end"]
	if start != "" {
		n, err := strconv.ParseUint(start, 10, 64)
		if err != nil {
			return chunk, "", fmt.Errorf("invalid Start")
		}
		chunk.Start = bgzf.Address(n)
	}

	if end != "" {
		n, err := strconv.ParseUint(end, 10, 64)
		if err != nil {
			return chunk, "", fmt.Errorf("invalid End")
		}
		chunk.End = bgzf.Address(n)
	}
	return chunk, id, nil
}
