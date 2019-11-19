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

// Package api implements the htsget readset retrieval API.
//
// The version implemented by this package is v1.0.0 defined at:
// http://samtools.github.io/hts-specs/htsget.html.
package api

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/googlegenomics/htsget/internal/analytics"
	"github.com/googlegenomics/htsget/internal/bam"
	"github.com/googlegenomics/htsget/internal/bgzf"
	"github.com/googlegenomics/htsget/internal/genomics"
)

const (
	readsPath = "/reads/"
	blockPath = "/block/"

	eofMarkerDataURL = "data:;base64,H4sIBAAAAAAA/wYAQkMCABsAAwAAAAAAAAAAAA=="
)

var (
	errInvalidOrUnspecifiedID = errors.New("invalid or unspecified ID")
	errNoFormatSpecified      = errors.New("no format specified")
	errMissingReferenceName   = errors.New("no reference name specified")
	errMissingOrInvalidToken  = errors.New("missing or invalid token")
)

// NewStorageClientFunc is the type of function that constructs the appropriate
// storage.Client to satisfy the incoming request. Any headers that caused this
// particular client to be created are returned to allow block requests to be
// generated correctly.
type NewStorageClientFunc func(*http.Request) (Client, http.Header, error)

// Server provides an htsget protocol server.  Must be created with NewServer.
type Server struct {
	newStorageClient NewStorageClientFunc
	blockSizeLimit   uint64
	whitelist        map[string]bool
}

// NewServer returns a new Server configured to use newStorageClient and
// blockSizeLimit. The server will call storageClientFunc on each request to
// determine which GCS storage client to use.
func NewServer(newStorageClient NewStorageClientFunc, blockSizeLimit uint64) *Server {
	return &Server{newStorageClient, blockSizeLimit, make(map[string]bool)}
}

// Whitelist adds buckets to the set of buckets which the server is allowed to
// access. If Whitelist is never called for a given Server then reads from any
// bucket are allowed.
func (server *Server) Whitelist(buckets []string) {
	for _, bucket := range buckets {
		server.whitelist[bucket] = true
	}
}

// Export registers the htsget API endpoint with mux and reads data using gcs.
// Blocks returned from the endpoint will generally not exceed blockSizeLimit
// bytes, though BAM chunks that already exceed this size will not be split.
func (server *Server) Export(mux *http.ServeMux) {
	mux.Handle(readsPath, forwardOrigin(server.serveReads))
	mux.Handle(blockPath, forwardOrigin(server.serveBlocks))
}

func (server *Server) serveReads(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	track := analytics.TrackerFromContext(ctx)
	track(analytics.Event("Reads", "Reads Request Received", "", nil))

	query := req.URL.Query()
	if err := parseFormat(query.Get("format")); err != nil {
		writeError(w, newUnsupportedFormatError(err))
		return
	}

	bucket, object, err := parseID(req.URL.Path[len(readsPath):])
	if err != nil {
		writeError(w, newInvalidInputError("parsing readset ID", err))
		return
	}

	if err := server.checkWhitelist(bucket); err != nil {
		writeError(w, newPermissionDeniedError("checking whitelist", err))
		return
	}

	gcs, headers, err := server.newStorageClient(req)
	if err != nil {
		writeError(w, newStorageError("creating client", err))
		return
	}

	data, err := gcs.NewObjectHandle(bucket, object).NewRangeReader(ctx, 0, int64(server.blockSizeLimit))
	if err != nil {
		writeError(w, newStorageError("opening data", err))
		return
	}
	defer data.Close()

	region, err := parseRegion(query, data)
	if err != nil {
		writeError(w, newInvalidInputError("parsing region", err))
		return
	}

	if region.End > 0 && region.Start > region.End {
		writeError(w, newInvalidRangeError(fmt.Errorf("%s: start > end", region)))
		return
	}

	request := &readsRequest{
		indexObjects: []ObjectHandle{
			gcs.NewObjectHandle(bucket, object+".bai"),
			gcs.NewObjectHandle(bucket, strings.TrimSuffix(object, ".bam")+".bai"),
		},
		blockSizeLimit: server.blockSizeLimit,
		region:         region,
	}

	chunks, err := request.handle(ctx)
	if err != nil {
		track(analytics.Event("Reads", "Reads Internal Error", "", nil))
		writeError(w, err)
		return
	}

	var base string
	if req.Host != "" {
		if req.TLS != nil {
			base = "https://"
		} else {
			base = "http://"
		}
		base += req.Host
	}
	base += strings.Replace(req.URL.Path, readsPath, blockPath, 1)

	var urls []map[string]interface{}
	for _, chunk := range chunks {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(chunk); err != nil {
			writeError(w, fmt.Errorf("encoding chunk: %v", err))
			return
		}

		url := map[string]interface{}{
			"url": fmt.Sprintf("%s?%s", base, base64.URLEncoding.EncodeToString(buf.Bytes())),
		}
		if len(headers) > 0 {
			// The htsget specification does not support multiple values for a single
			// header.
			flattened := make(map[string]string)
			for k, v := range headers {
				flattened[k] = v[0]
			}
			url["headers"] = flattened
		}
		urls = append(urls, url)
	}
	urls = append(urls, map[string]interface{}{"url": eofMarkerDataURL})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"htsget": map[string]interface{}{
			"format": "BAM",
			"urls":   urls,
		}})

	count := int64(len(urls))
	track(analytics.Event("Reads", "Reads Response URL Count", "", &count))
	track(analytics.Event("Reads", "Reads Response Sent", "", nil))
}

func (server *Server) serveBlocks(w http.ResponseWriter, req *http.Request) {
	bucket, object, err := parseID(req.URL.Path[len(blockPath):])
	if err != nil {
		writeError(w, newInvalidInputError("parsing readset ID", err))
		return
	}

	if err := server.checkWhitelist(bucket); err != nil {
		writeError(w, newPermissionDeniedError("checking whitelist", err))
		return
	}

	var chunk bgzf.Chunk
	if err := decodeRawQuery(req.URL.RawQuery, &chunk); err != nil {
		writeError(w, fmt.Errorf("decoding raw query: %v", err))
		return
	}

	gcs, _, err := server.newStorageClient(req)
	if err != nil {
		writeError(w, fmt.Errorf("creating storage client: %v", err))
		return
	}

	request := &blockRequest{
		object: gcs.NewObjectHandle(bucket, object),
		chunk:  chunk,
	}

	response, err := request.handle(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	defer response.Close()

	w.Header().Add("Content-type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, response); err != nil {
		log.Printf("Failed to copy response: %v", err)
		return
	}
}

func (server *Server) checkWhitelist(bucket string) error {
	if len(server.whitelist) == 0 || server.whitelist[bucket] {
		return nil
	}
	return fmt.Errorf("access to bucket %s is not allowed", bucket)
}

func decodeRawQuery(rawQuery string, v interface{}) error {
	b, err := base64.URLEncoding.DecodeString(rawQuery)
	if err != nil {
		return fmt.Errorf("base64: %v", err)
	}

	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(v); err != nil {
		return fmt.Errorf("gob: %v", err)
	}

	return nil
}

// parseID parses path and returns a GCS bucket and object, or an error.
func parseID(path string) (string, string, error) {
	if parts := strings.SplitN(path, "/", 2); len(parts) == 2 {
		if parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], nil
		}
	}
	return "", "", errInvalidOrUnspecifiedID
}

func parseFormat(format string) error {
	if format != "" && format != "BAM" {
		return fmt.Errorf("unsupported format %q", format)
	}
	return nil
}

func parseRegion(query url.Values, data io.Reader) (genomics.Region, error) {
	var (
		name  = query.Get("referenceName")
		start = query.Get("start")
		end   = query.Get("end")
	)
	if name == "" && start == "" && end == "" {
		return genomics.AllMappedReads, nil
	}
	if name == "" {
		return genomics.Region{}, errMissingReferenceName
	}

	id, err := bam.GetReferenceID(data, name)
	if err != nil {
		return genomics.Region{}, fmt.Errorf("resolving reference %q: %v", name, err)
	}

	region := genomics.Region{ReferenceID: id}

	if start != "" {
		n, err := strconv.ParseUint(start, 10, 32)
		if err != nil {
			return genomics.Region{}, fmt.Errorf("parsing start: %v", err)
		}
		region.Start = uint32(n)
	}

	if end != "" {
		n, err := strconv.ParseUint(end, 10, 32)
		if err != nil {
			return genomics.Region{}, fmt.Errorf("parsing end: %v", err)
		}
		region.End = uint32(n)
	}

	return region, nil
}

// apiError is used to capture errors that have been defined in the API.
type apiError struct {
	name  string
	code  int
	cause error
}

func (err *apiError) Error() string {
	return fmt.Sprintf("%s (%d): %v", err.name, err.code, err.cause)
}

func newApiError(name string, code int, context string, err error) error {
	return &apiError{name, code, fmt.Errorf("%s: %v", context, err)}
}

func newInvalidAuthenticationError(context string, err error) error {
	return newApiError("InvalidAuthentication", http.StatusUnauthorized, context, err)
}

func newInvalidInputError(context string, err error) error {
	return newApiError("InvalidInput", http.StatusBadRequest, context, err)
}

func newInvalidRangeError(err error) error {
	return &apiError{"InvalidRange", http.StatusBadRequest, err}
}

func newPermissionDeniedError(context string, err error) error {
	return newApiError("PermissionDenied", http.StatusForbidden, context, err)
}

func newUnsupportedFormatError(err error) error {
	return &apiError{"UnsupportedFormat", http.StatusBadRequest, err}
}

func newNotFoundError(context string, err error) error {
	return newApiError("NotFound", http.StatusNotFound, context, err)
}

// writeError writes either a JSON object or bare HTTP error describing err to
// w.  A JSON object is written only when the error has a name and code defined
// by the htsget specification.
func writeError(w http.ResponseWriter, err error) {
	if err, ok := err.(*apiError); ok {
		writeJSON(w, err.code, map[string]interface{}{
			"error":   err.name,
			"message": fmt.Sprintf("%s: %v", http.StatusText(err.code), err.cause),
		})
		return
	}

	writeHTTPError(w, http.StatusInternalServerError, err)
}

func writeHTTPError(w http.ResponseWriter, code int, err error) {
	http.Error(w, fmt.Sprintf("%s: %v", http.StatusText(code), err), code)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

type forwardOrigin func(w http.ResponseWriter, req *http.Request)

func (f forwardOrigin) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	f(w, req)
}
