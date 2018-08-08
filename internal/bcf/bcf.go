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

package bcf

import (
	"bufio"
	"compress/gzip"
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

	if err := binary.CheckMagic(gzr, []byte(bcfMagic)); err != nil {
		return 0, fmt.Errorf("checking magic of BCF file: %v", err)
	}

	var length uint32
	if err := binary.Read(gzr, &length); err != nil {
		return 0, fmt.Errorf("reading header length: %v", err)
	}

	headerReader := io.LimitReader(gzr, int64(length))
	scanner := bufio.NewScanner(headerReader)
	var id int
	var contigsFound bool
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "##contig") {
			contigsFound = true
			if contigDefinesReference(line, referenceName) {
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
			if contigsFound {
				return 0, fmt.Errorf("reference name not found")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scanning header: %v", err)
	}
	return 0, fmt.Errorf("region id not found")
}

func contigDefinesReference(contig, refName string) bool {
	index := strings.Index(contig, fmt.Sprintf("ID=%s", refName))
	if index == -1 {
		return false
	}
	if nextChr := contig[index+len("ID=")+len(refName)]; nextChr != ',' && nextChr != '>' {
		return false
	}
	return true
}

func getIdx(contig string) (int, error) {
	index := strings.Index(contig, "IDX=")
	if index == -1 {
		return -1, nil
	}
	index += len("IDX=")
	var buff []byte
	for n := len(contig); index < n; index++ {
		chr := contig[index]
		if chr == ',' || chr == '>' {
			break
		}
		buff = append(buff, chr)
	}
	idx, err := strconv.Atoi(string(buff))
	if err != nil {
		return -1, fmt.Errorf("parsing IDX value: %v", err)
	}
	return idx, nil
}
