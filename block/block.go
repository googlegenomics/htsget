package block

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/googlegenomics/htsget/internal/bgzf"
)

//RangeReader takes in a start and a length and return a read closer that reads length from the start
type RangeReader func(start int64, length int64) (io.ReadCloser, error)

//ReadCloser has one reader and multiple closers
type ReadCloser struct {
	io.Reader
	io.Closer
}

func (m ReadCloser) Read(b []byte) (int, error) {
	return m.Reader.Read(b)
}

//Close closes files
func (m ReadCloser) Close() error {
	return m.Closer.Close()
}

type multiCloser struct {
	closers []io.Closer
}

func (m multiCloser) Close() error {
	errs := make([]error, 0)
	for _, v := range m.closers {
		if err := v.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("MultiReadFileCloser: error closing files: %s", errs)
	}
	return nil
}

// ReadBlock read block take in a file and a chunk and returns a read closer to read out the value of a bam chunks
func ReadBlock(file RangeReader, chunk bgzf.Chunk) (io.ReadCloser, error) {
	start, end := chunk.Start, chunk.End
	head, tail := int64(start.BlockOffset()), int64(end.BlockOffset())

	// The simple (unlikely) case is when the chunk resides in a single block.
	if head == tail {
		// block, err := req.object.NewRangeReader(ctx, head, bgzf.MaximumBlockSize)
		block, err := file(head, bgzf.MaximumBlockSize)
		// defer block.Close()
		decoded, _, err := bgzf.DecodeBlock(block)
		if err != nil {
			return nil, fmt.Errorf("decoding block: %v", err)
		}
		decoded = decoded[start.DataOffset():end.DataOffset()]

		encoded, err := bgzf.EncodeBlock(decoded)
		if err != nil {
			return nil, fmt.Errorf("encoding prefix: %v", err)
		}
		return ioutil.NopCloser(bytes.NewReader(encoded)), nil
	}

	var readers []io.Reader
	var closers []io.Closer

	// Read the first block and reconstruct a prefix block.
	if start.DataOffset() != 0 {
		first, err := file(head, tail-head)
		// defer first.Close()

		decoded, length, err := bgzf.DecodeBlock(first)
		if err != nil {
			return nil, fmt.Errorf("decoding first block: %v", err)
		}

		head += int64(length)

		encoded, err := bgzf.EncodeBlock(decoded[start.DataOffset():])
		if err != nil {
			return nil, fmt.Errorf("encoding prefix: %v", err)
		}
		readers = append(readers, ioutil.NopCloser(bytes.NewReader(encoded)))
		closers = append(closers, first)
	}

	// Read any intermediate blocks (no modification needed).
	if tail-head > 0 {
		r, err := file(head, tail-head)
		if err != nil {
			return nil, err
		}
		readers = append(readers, r)
		closers = append(closers, r)
	}

	// Read the last block and reconstruct a suffix block.
	theEndBlock := end.DataOffset()
	if theEndBlock != 0 {
		last, err := file(head, tail-head)
		if err != nil {
			return nil, err
		}

		decoded, _, err := bgzf.DecodeBlock(last)
		if err != nil {
			return nil, fmt.Errorf("decoding last block: %v", err)
		}
		encoded, err := bgzf.EncodeBlock(decoded[:end.DataOffset()])
		if err != nil {
			return nil, fmt.Errorf("encoding suffix: %v", err)
		}
		readers = append(readers, ioutil.NopCloser(bytes.NewReader(encoded)))
		closers = append(closers, last)
	}

	return &ReadCloser{
		Reader: io.MultiReader(readers...),
		Closer: &multiCloser{closers},
	}, nil
}
