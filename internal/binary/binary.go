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

// Package binary provides support for operating on binary data.
package binary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// ExpectBytes reads len(want) bytes from r and returns an error on mismatch.
func ExpectBytes(r io.Reader, want []byte) error {
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		return fmt.Errorf("reading bytes: %v", err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("comparing bytes: got %q, want %q", got, want)
	}
	return nil
}

// Read reads a little endian value from r into v using binary.Read.
func Read(r io.Reader, v interface{}) error {
	return binary.Read(r, binary.LittleEndian, v)
}
