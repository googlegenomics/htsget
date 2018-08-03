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

package common

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ChecksMagic checks the magic bytes from the provided reader.
func CheckMagic(r io.Reader, want []byte) error {
	got := make([]byte, len(want))
	if err := Read(r, &got); err != nil {
		return fmt.Errorf("reading magic: %v", err)
	}
	for i, n := 0, len(want); i < n; i++ {
		if got[i] != want[i] {
			return fmt.Errorf("wrong magic %v (wanted %v)", got, want)
		}
	}
	return nil
}

// Read reads the value from the provided reader into the provided interface.
func Read(r io.Reader, v interface{}) error {
	return binary.Read(r, binary.LittleEndian, v)
}
