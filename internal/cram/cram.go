// Package cram provides support for parsing CRAM files.
package cram

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/googlegenomics/htsget/internal/sam"
)

type fileDefinition struct {
	Magic        uint32
	MajorVersion uint8
	MinorVersion uint8
	ID           [20]byte
}

type blockHeader struct {
	Method      byte
	ContentType byte
	ContentID   int32
	Length      int32
	RawLength   int32
}

const (
	// Magic number for identifying CRAM files.
	magic = 0x4d415243
)

// GetReferenceID returns the ID of the provided reference name from a CRAM file.
func GetReferenceID(r io.Reader, reference string) (int32, error) {
	var cram fileDefinition
	if err := read(r, &cram); err != nil {
		return 0, fmt.Errorf("reading file definition: %v", err)
	}
	if cram.Magic != magic {
		return 0, fmt.Errorf("invalid magic value, got: %08x, want: %08x", cram.Magic, magic)
	}

	if err := cram.skipContainerHeader(r); err != nil {
		return 0, fmt.Errorf("reading container header: %v", err)
	}

	bh, err := cram.readblockHeader(r)
	if err != nil {
		return 0, fmt.Errorf("reading block header: %v", err)
	}

	if bh.Method == 1 {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return 0, fmt.Errorf("reading gzipped header: %v", err)
		}

		// Without this, the gzip reader may read past the end of the header archive.
		gz.Multistream(false)
		r = gz
	}

	var limit int32
	if err := read(r, &limit); err != nil {
		return 0, fmt.Errorf("reading header length: %v", err)
	}
	r = io.LimitReader(r, int64(limit))

	id, err := sam.GetReferenceID(r, reference)
	if err != nil {
		return 0, fmt.Errorf("getting reference ID: %v", err)
	}
	return id, nil
}

func (cram *fileDefinition) skipContainerHeader(r io.Reader) error {
	var skip int32
	if err := read(r, &skip); err != nil {
		return fmt.Errorf("skipping length: %v", err)
	}

	for i := 0; i < 7; i++ {
		if err := readITF8(r, &skip); err != nil {
			return fmt.Errorf("skipping header field: %v", err)
		}
	}

	var landmarkCount int32
	if err := readITF8(r, &landmarkCount); err != nil {
		return fmt.Errorf("skipping landmark count: %v", err)
	}
	for i := 0; i < int(landmarkCount); i++ {
		if err := readITF8(r, &skip); err != nil {
			return fmt.Errorf("skipping landmark %d: %v", i, err)
		}
	}

	if cram.MajorVersion >= 3 {
		if err := read(r, &skip); err != nil {
			return fmt.Errorf("skipping CRC: %v", err)
		}
	}

	return nil
}

func (cram *fileDefinition) readblockHeader(r io.Reader) (*blockHeader, error) {
	var block blockHeader
	if err := read(r, &block.Method); err != nil {
		return nil, fmt.Errorf("reading method: %v", err)
	}
	if err := read(r, &block.ContentType); err != nil {
		return nil, fmt.Errorf("reading content type: %v", err)
	}

	if err := readITF8(r, &block.ContentID); err != nil {
		return nil, fmt.Errorf("reading content ID: %v", err)
	}
	if err := readITF8(r, &block.Length); err != nil {
		return nil, fmt.Errorf("reading length: %v", err)
	}
	if err := readITF8(r, &block.RawLength); err != nil {
		return nil, fmt.Errorf("reading raw length: %v", err)
	}

	return &block, nil
}

func readITF8(r io.Reader, i *int32) error {
	bytes := make([]byte, 1, 5)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return fmt.Errorf("reading first byte: %v", err)
	}

	bytes = bytes[:countLeadingOnes(bytes[0])+1]
	if _, err := io.ReadFull(r, bytes[1:]); err != nil {
		return fmt.Errorf("reading remaining bytes: %v", err)
	}

	switch n := len(bytes); n {
	case 1:
		*i = int32(bytes[0])
	case 2:
		*i = int32(uint32(bytes[0]&0x7f)<<8 | uint32(bytes[1]))
	case 3:
		*i = int32(uint32(bytes[0]&0x3f)<<16 | uint32(bytes[1])<<8 | uint32(bytes[2]))
	case 4:
		*i = int32(uint32(bytes[0]&0x1f)<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3]))
	case 5:
		*i = int32(uint32(bytes[0]&0x0f)<<28 | uint32(bytes[1])<<20 | uint32(bytes[2])<<12 | uint32(bytes[3])<<4 | uint32(bytes[4]&0x0f))
	default:
		panic(fmt.Sprintf("invalid ITF8 length: %d", n))
	}

	return nil
}

func countLeadingOnes(b byte) int {
	for i := 0; i < 4; i++ {
		if b&0x80 == 0 {
			return i
		}
		b <<= 1
	}
	return 4
}

func read(r io.Reader, v interface{}) error {
	return binary.Read(r, binary.LittleEndian, v)
}
