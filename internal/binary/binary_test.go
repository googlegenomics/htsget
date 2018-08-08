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

package binary

import (
	"bytes"
	"testing"
)

func TestExpectBytes(t *testing.T) {
	testCases := []struct {
		want  []byte
		input []byte
		match bool
	}{
		{[]byte("BCF\x02\x02"), []byte("BCF\x02\x02"), true},
		{[]byte("BCF\x02\x02"), []byte("BCF\x02\x02EXTRA"), true},
		{[]byte("BCF\x02\x02"), []byte("BCF\x03\x02"), false},
		{[]byte("BCF\x02\x02"), []byte("BCF\x02"), false},
		{[]byte("BCF\x02\x02"), []byte(""), false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.input), func(t *testing.T) {
			err := ExpectBytes(bytes.NewReader(tc.input), tc.want)
			if err != nil && tc.match {
				t.Fatalf("ExpectBytes returned unexpected error: %v", err)
			} else if err == nil && !tc.match {
				t.Fatalf("ExpectBytes accepted mismatched input %v", tc.match)
			}
		})
	}
}
