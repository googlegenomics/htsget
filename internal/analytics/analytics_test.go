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

package analytics

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
)

func TestClient_Send_Batches(t *testing.T) {
	var requests int
	client, quit := fakeBackend(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	})
	defer close(quit)

	var hits []Hit
	for i := 0; i < client.batchSize*4; i++ {
		hits = append(hits, Event("tests", "test", "", nil))
	}
	client.Send(hits)
	if got, want := requests, len(hits)/client.batchSize; got != want {
		t.Errorf("Wrong number of requests: got %d, want %d", got, want)
	}
}

func TestClient_Send_VerifyPayloads(t *testing.T) {
	var payloads []string

	client, quit := fakeBackend(func(w http.ResponseWriter, req *http.Request) {
		scanner := bufio.NewScanner(req.Body)
		for scanner.Scan() {
			payloads = append(payloads, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	})
	defer close(quit)

	var hits []Hit
	for i := int64(0); i < 10; i++ {
		hits = append(hits, Event("tests", "test", fmt.Sprintf("%d", i), &i))
	}

	if err := client.Send(hits); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	for i, payload := range payloads {
		got, err := url.ParseQuery(payload)
		if err != nil {
			t.Errorf("Failed to parse payload: %q: %v", payload, err)
		}

		want := url.Values{
			"v":   []string{"1"},
			"cid": []string{client.clientID},
			"tid": []string{client.propertyID},
		}
		for key, value := range hits[i] {
			want.Add(key, value)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("Wrong payload for hit %d: got %v, want %v", i, got, want)
		}
	}
}

func TestEvent_TypeParameter(t *testing.T) {
	if got, want := Event("tests", "test", "", nil)["t"], "event"; got != want {
		t.Errorf("Wrong hit type: got %q, want %q", got, want)
	}
}

func TestEvent_OptionalParameters(t *testing.T) {
	if _, ok := Event("tests", "test", "", nil)["el"]; ok {
		t.Error("Label parameter was added for empty label")
	}
	if _, ok := Event("tests", "test", "", nil)["ev"]; ok {
		t.Error("Value parameter was added for empty label")
	}
}

func TestEvent_Values(t *testing.T) {
	testcases := []struct {
		name  string
		value int64
		want  string
	}{
		{"zero", 0, "0"},
		{"maximum", math.MaxInt64, strconv.Itoa(math.MaxInt64)},
		{"minimum", math.MinInt64, strconv.Itoa(math.MinInt64)},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Event("tests", "test", "", &tc.value)["ev"]; got != tc.want {
				t.Fatalf("Wrong value: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTrackingHandler(t *testing.T) {
	want := []Hit{
		Event("tests", "test", "a", nil),
		Event("tests", "test", "b", nil),
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		track := TrackerFromContext(req.Context())
		for i := range want {
			track(want[i])
		}
	})

	var invoked bool
	tracker := func(got []Hit) {
		if len(got) != len(want) {
			t.Fatalf("Wrong number of hits: got %d, want %d", len(got), len(want))
		}
		for i := range want {
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Errorf("Hit %d: got %v, want %v", i, got[i], want[i])
			}
		}
		invoked = true
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	TrackingHandler(handler, tracker).ServeHTTP(w, req)

	if !invoked {
		t.Error("tracker function was not invoked")
	}
}

func TestTrackerFromContext_WithEmptyContextIsNotNil(t *testing.T) {
	ctx := context.Background()
	if track := TrackerFromContext(ctx); track == nil {
		t.Error("TrackerFromContext returned nil")
	}
}

func fakeBackend(handler http.HandlerFunc) (*Client, chan<- struct{}) {
	server := httptest.NewServer(handler)
	quit := make(chan struct{})
	go func() {
		<-quit
		server.Close()
	}()

	client := NewClient("UA-TEST123", "0001-0002-0003-0004")
	client.endpoint = server.URL

	return client, quit
}
