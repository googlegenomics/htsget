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

// Package analytics provides functions for sending data to Google Analytics.
package analytics

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	defaultEndpoint  = "https://www.google-analytics.com/"
	defaultBatchSize = 20 // The maximum number supported by batch endpoint.
)

// Hit represents a single analytics event (called a 'hit').
type Hit map[string]string

// Event generates a new event typed hit.  The label may be empty and the
// value may be nil but category and action are required.
func Event(category, action, label string, value *int64) Hit {
	hit := Hit{
		"t":  "event",
		"ec": category,
		"ea": action,
	}
	if label != "" {
		hit["el"] = label
	}
	if value != nil {
		hit["ev"] = strconv.FormatInt(*value, 10)
	}
	return hit
}

// Client defines a type for communicating with Google Analytics.  To create a
// properly initialized Client instance, use NewClient.
type Client struct {
	propertyID string
	clientID   string
	endpoint   string
	batchSize  int
}

// NewClient returns a Client sends hits to analytics using the provided IDs.
func NewClient(propertyID, clientID string) *Client {
	return &Client{propertyID, clientID, defaultEndpoint, defaultBatchSize}
}

// Send attempts to upload the provided hits to the analytics server.
func (client *Client) Send(hits []Hit) error {
	if len(hits) > 0 {
		if err := client.upload(hits); err != nil {
			return fmt.Errorf("uploading hits: %v", err)
		}
	}
	return nil
}

func (c *Client) upload(hits []Hit) error {
	for i := 0; i < len(hits); i += c.batchSize {
		start, end := i, i+c.batchSize
		if end > len(hits) {
			end = len(hits)
		}

		var body bytes.Buffer
		for _, hit := range hits[start:end] {
			payload := url.Values{
				"v":   []string{"1"},
				"tid": []string{c.propertyID},
				"cid": []string{c.clientID},
			}
			for key, value := range hit {
				payload.Add(key, value)
			}
			body.WriteString(payload.Encode())
			body.WriteByte('\n')
		}

		request, err := http.NewRequest("POST", c.endpoint+"/batch", &body)
		if err != nil {
			return fmt.Errorf("creating request: %v", err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return fmt.Errorf("sending request: %v", err)
		}
		if response.StatusCode != 200 {
			return fmt.Errorf("unexpected response status: %v", response.Status)
		}
	}
	return nil
}

type contextKey int

var (
	hitsKey = contextKey(1)
)

// TrackingHandler returns a new http.Handler which wraps the provided
// handler.  The wrapper prepares the incoming request's context for use with
// the TrackerFromContext function.  When the underlying handler completes,
// the track function is invoked with any hits accumulated during the request.
func TrackingHandler(handler http.Handler, track func([]Hit)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var hits []Hit
		ctx := context.WithValue(req.Context(), hitsKey, &hits)
		handler.ServeHTTP(w, req.WithContext(ctx))
		track(hits)
	})
}

// TrackerFromContext is intended to be used with contexts that are generated
// by handlers returned from the TrackingHandler function.  It returns a
// function that buffers hits to be delivered to the track function provided
// in the original call to the TrackingHandler function.
func TrackerFromContext(ctx context.Context) func(Hit) {
	if hits, ok := ctx.Value(hitsKey).(*[]Hit); ok {
		return func(hit Hit) { *hits = append(*hits, hit) }
	}
	return func(Hit) {}
}
