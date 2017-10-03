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
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/cram"
	"github.com/googlegenomics/htsget/internal/genomics"
)

type readsRequest struct {
	indexObject    *storage.ObjectHandle
	blockSizeLimit uint64
	region         genomics.Region
}

func (req *readsRequest) handleBAM(ctx context.Context) ([]interface{}, error) {
	index, err := req.indexObject.NewReader(ctx)
	if err != nil {
		return nil, newStorageError("opening index", err)
	}
	defer index.Close()

	chunks, err := bam.Read(index, req.region)
	if err != nil {
		return nil, fmt.Errorf("reading index: %v", err)
	}

	var ret []interface{}
	for _, c := range bgzf.Merge(chunks, req.blockSizeLimit) {
		ret = append(ret, c)
	}
	return ret, nil
}

func (req *readsRequest) handleCRAM(ctx context.Context) ([]interface{}, error) {
	crai, err := req.indexObject.NewReader(ctx)
	if err != nil {
		return nil, newStorageError("opening index", err)
	}
	defer crai.Close()

	index, err := cram.ReadIndex(crai)
	if err != nil {
		return nil, newStorageError("reading index", err)
	}

	chunks := index.GetChunksForRegion(req.region)

	var ret []interface{}
	for _, c := range cram.SortAndMerge(chunks, req.blockSizeLimit) {
		ret = append(ret, c)
	}
	return ret, nil
}
