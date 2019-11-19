// Copyright 2017 Google Inc.
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

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/googlegenomics/htsget/internal/bgzf"
)

type blockRequest struct {
	object ObjectHandle
	chunk  bgzf.Chunk
}

func (req *blockRequest) handle(ctx context.Context) (io.ReadCloser, error) {
	start, end := req.chunk.Start, req.chunk.End
	head, tail := int64(start.BlockOffset()), int64(end.BlockOffset())

	// The simple (unlikely) case is when the chunk resides in a single block.
	if head == tail {
		block, err := req.object.NewRangeReader(ctx, head, bgzf.MaximumBlockSize)
		if err != nil {
			return nil, newStorageError("opening block", err)
		}
		defer block.Close()

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
		first, err := req.object.NewRangeReader(ctx, head, bgzf.MaximumBlockSize)
		if err != nil {
			return nil, newStorageError("opening first block", err)
		}
		defer first.Close()

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
	}

	// Read any intermediate blocks (no modification needed).
	if tail-head > 0 {
		r, err := req.object.NewRangeReader(ctx, head, tail-head)
		if err != nil {
			return nil, newStorageError("opening body block", err)
		}
		readers = append(readers, r)
		closers = append(closers, r)
	}

	// Read the last block and reconstruct a suffix block.
	if end.DataOffset() != 0 {
		last, err := req.object.NewRangeReader(ctx, tail, bgzf.MaximumBlockSize)
		if err != nil {
			return nil, newStorageError("opening last block", err)
		}
		defer last.Close()

		decoded, _, err := bgzf.DecodeBlock(last)
		if err != nil {
			return nil, fmt.Errorf("decoding last block: %v", err)
		}
		encoded, err := bgzf.EncodeBlock(decoded[:end.DataOffset()])
		if err != nil {
			return nil, fmt.Errorf("encoding suffix: %v", err)
		}
		readers = append(readers, ioutil.NopCloser(bytes.NewReader(encoded)))
	}

	return &multiReadCloser{
		Reader:  io.MultiReader(readers...),
		closers: closers,
	}, nil
}

type multiReadCloser struct {
	io.Reader

	closers []io.Closer
}

func (mrc *multiReadCloser) Close() error {
	var errors []error
	for _, closer := range mrc.closers {
		if err := closer.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("one or more errors: %v", errors)
	}
	return nil
}
