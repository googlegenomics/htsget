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

// Package bcf contains support for parsing BCF files.
package bcf

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/googlegenomics/htsget/internal/binary"
)

const (
	bcfMagic = "BCF\x02\x02"
)

// GetReferenceID retrieves the reference id of the given referenceName
// from the provided bcf file.
func GetReferenceID(bcf io.Reader, referenceName string) (int, error) {
	gzr, err := gzip.NewReader(bcf)
	if err != nil {
		return 0, fmt.Errorf("initializing gzip reader: %v", err)
	}
	defer gzr.Close()

	if err := binary.ExpectBytes(gzr, []byte(bcfMagic)); err != nil {
		return 0, fmt.Errorf("checking magic: %v", err)
	}

	var length uint32
	if err := binary.Read(gzr, &length); err != nil {
		return 0, fmt.Errorf("reading header length: %v", err)
	}

	headerReader := io.LimitReader(gzr, int64(length))
	scanner := bufio.NewScanner(headerReader)
	var id int
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "##contig") {
			if contigField(line, "ID") == referenceName {
				idx, err := getIdx(line)
				if err != nil {
					return 0, fmt.Errorf("getting idx: %v", err)
				}
				if idx > -1 {
					return idx, nil
				}
				return id, nil
			}
			id++
		} else {
			if id > 0 {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scanning header: %v", err)
	}
	return 0, errors.New("reference name not found")
}

func contigField(input, name string) string {
	field := fmt.Sprintf("%s=", name)
	for {
		start := strings.Index(input, field)
		if start == -1 {
			return ""
		}
		if start > 0 && !isDelimiter(input[start-1]) {
			input = input[start+len(field):]
			continue
		}
		input = input[start+len(field):]
		if end := strings.IndexAny(input, ",>"); end > 0 {
			return input[:end]
		} else {
			return input
		}
	}
}

func isDelimiter(chr byte) bool {
	return chr == ',' || chr == '<'
}

func getIdx(contig string) (int, error) {
	idx := contigField(contig, "IDX")
	if idx == "" {
		return -1, nil
	}
	return strconv.Atoi(string(idx))
}
