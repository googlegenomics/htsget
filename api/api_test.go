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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

const (
	testBlockSizeLimit = 32 * 1024 // Small block size for small test data.
)

func TestInvalidInputs(t *testing.T) {
	testCases := []struct{ name, url string }{
		{"no readset ID or parameters", "/reads/"},
		{"missing readset ID", "/reads/?format=BAM"},
		{"invalid ID (no object)", "/reads/bucket?format=BAM"},
		{"invalid ID (trailing slash, no object)", "/reads/bucket/?format=BAM"},
	}
	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectError(t, "InvalidInput", http.StatusBadRequest,
				testQuery(ctx, t, tc.url))
		})
	}
}

func TestUnsupportedFormats(t *testing.T) {
	testCases := []struct{ name, url string }{
		{"unknown format", "/reads/bucket/object?format=XYZ"},
		{"cram format", "/reads/bucket/object?format=CRAM"},
		{"lowercase bam", "/reads/bucket/object?format=bam"},
	}
	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectError(t, "UnsupportedFormat", http.StatusBadRequest,
				testQuery(ctx, t, tc.url))
		})
	}
}

func TestMissingObject(t *testing.T) {
	ctx := context.Background()
	expectError(t, "NotFound", http.StatusNotFound,
		testQuery(ctx, t, "/reads/foo/bar"))
}

func TestSimpleRead(t *testing.T) {
	fakeClient := &http.Client{Transport: &fakeGCS{t}}
	ctx := context.WithValue(context.Background(), testHTTPClientKey, fakeClient)
	resp := testQuery(ctx, t, "/reads/testdata/NA12878.chr20.sample.bam")

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Wrong status code: got %v, want %v", got, want)
	}

	var body struct {
		URLs []struct {
			URL string `json:"url"`
		} `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	for _, url := range body.URLs {
		if url.URL == eofMarkerDataURL {
			continue
		}

		resp := testQuery(ctx, t, url.URL)
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("Wrong status code: got %v, want %v", got, want)
			continue
		}
		length, err := io.Copy(ioutil.Discard, resp.Body)
		if err != nil {
			t.Errorf("Failed to read response body: %v", err)
			continue
		}
		if got, want := length, int64(testBlockSizeLimit); got > want {
			t.Errorf("Data block too large: got %v, want at most %v", got, want)
		}
	}
}

func TestShortNameIndexFile(t *testing.T) {
	fakeClient := &http.Client{Transport: &fakeGCS{t}}
	ctx := context.WithValue(context.Background(), testHTTPClientKey, fakeClient)

	resp := testQuery(ctx, t, "/reads/testdata/index.sample.bam")

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("Wrong status code: got %v, want %v", got, want)
	}
}

func TestNoIndexFiles(t *testing.T) {
	fakeClient := &http.Client{Transport: &fakeGCS{t}}
	ctx := context.WithValue(context.Background(), testHTTPClientKey, fakeClient)

	resp := testQuery(ctx, t, "/reads/testdata/noindex.sample.bam")

	if resp.StatusCode == http.StatusOK {
		t.Error("Read succeeded with missing index file")
	}
}

// This test ensures that the undocumented error handling behaviour of the GCS
// storage client does not change.
func TestGoogleAPIInternalErrors(t *testing.T) {
	testCases := []struct {
		name       string
		transport  http.RoundTripper
		statusCode int
	}{
		{"unauthorized", fixedStatus(http.StatusUnauthorized), http.StatusUnauthorized},
		{"forbidden", fixedStatus(http.StatusForbidden), http.StatusForbidden},
		{"not found", fixedStatus(http.StatusNotFound), http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: tc.transport}
			ctx := context.WithValue(context.Background(), testHTTPClientKey, client)
			resp := testQuery(ctx, t, "/reads/testdata/NA12878.chr20.sample.bam")
			if got, want := resp.StatusCode, tc.statusCode; got != want {
				t.Errorf("Wrong status code: got %v, want %v", got, want)
			}
		})
	}
}

type testContextKey int

var (
	testHTTPClientKey = testContextKey(0)
)

func testQuery(ctx context.Context, t *testing.T, url string) *http.Response {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to parse URL %q: %v", url, err)
	}
	req = req.WithContext(ctx)

	client, ok := ctx.Value(testHTTPClientKey).(*http.Client)
	if !ok {
		client = &http.Client{Transport: fixedStatus(http.StatusNotFound)}
	}

	gcs, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("Failed to create storage client: %v", err)
	}
	newStorageClient := func(*http.Request) (Client, http.Header, error) {
		return GCSClient{gcs}, nil, nil
	}

	mux := http.NewServeMux()
	server := NewServer(newStorageClient, testBlockSizeLimit)
	server.Export(mux)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	return w.Result()
}

func expectError(t *testing.T, name string, code int, resp *http.Response) {
	if got, want := resp.StatusCode, code; got != want {
		t.Errorf("Wrong status code: got %v, want %v", got, want)
	}
	body := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
	if got, want := body["error"], name; got != want {
		t.Errorf("Wrong 'error' field value: got %v, want %v", got, want)
	}
}

type fixedStatus int

func (code fixedStatus) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     http.StatusText(int(code)),
		StatusCode: int(code),
		Body:       http.NoBody,
	}, nil
}

type fakeGCS struct {
	*testing.T
}

func (fake *fakeGCS) RoundTrip(req *http.Request) (*http.Response, error) {
	filename := "testdata/" + path.Base(req.URL.Path)

	content, err := os.Open(filename)
	if err != nil {
		response := httptest.NewRecorder()
		http.Error(response, fmt.Sprintf("Failed to open test data: %v", err), http.StatusNotFound)
		return response.Result(), nil
	}
	defer content.Close()

	w := httptest.NewRecorder()
	http.ServeContent(w, req, filename, time.Now(), content)
	return w.Result(), nil
}
