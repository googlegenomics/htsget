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

// This binary provides an  htsget client that supports Google authentication.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	scope = "https://www.googleapis.com/auth/devstorage.read_only"
)

var (
	reference = flag.String("r", "", "reference name")
	output    = flag.String("o", "", "output filename")
)

func main() {
	flag.Parse()

	w := io.Writer(os.Stdout)
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			log.Fatalf("Failed to open output file: %v", err)
		}
		defer f.Close()

		w = f
	}

	ctx := context.Background()

	// For compatibility with other tools, read the standard cURL certificate
	// authority override from the environment.
	if bundle := os.Getenv("CURL_CA_BUNDLE"); bundle != "" {
		pem, err := ioutil.ReadFile(bundle)
		if err != nil {
			log.Fatalf("Failed to read CA override file %q: %v", bundle, err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			log.Fatalf("Failed to initialize system certificate pool: %v", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			log.Fatalf("Failed to add certificates from bundle %q", bundle)
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: pool,
				}},
		})
		log.Printf("Using CA override bundle from %q", bundle)
	}

	client, err := google.DefaultClient(ctx, scope)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	for _, target := range flag.Args() {
		log.Printf("Fetching %q", target)
		if *reference != "" {
			target = addParameter(target, "referenceName", *reference)
		}
		resp, err := client.Get(target)
		if err != nil {
			log.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Unexpected response: %v", errorFromResponse(resp))
		}

		var ticket struct {
			Container struct {
				URLs []struct {
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
				} `json:"urls"`
			} `json:"htsget"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ticket); err != nil {
			log.Fatalf("Failed to decode response: %v", err)
		}

		log.Printf("Received ticket with %d URLs", len(ticket.Container.URLs))

		for i, blob := range ticket.Container.URLs {
			r, err := fetchBlob(ctx, blob.URL, blob.Headers)
			if err != nil {
				log.Fatalf("Blob %d: failed to fetch data: %v", i, err)
			}
			defer r.Close()

			n, err := io.Copy(w, r)
			if err != nil {
				log.Fatalf("Blob %d: copying data to disk", i, err)
			}
			log.Printf("Blob %d: wrote %d bytes", i, n)
		}
	}
}

func addParameter(input, name, value string) string {
	values := url.Values{}
	values.Set(name, value)
	if strings.Contains(input, "?") {
		return input + "&" + values.Encode()
	}
	return input + "?" + values.Encode()
}

func humanSize(n int64) string {
	kb := n / 1024
	mb := kb / 1024
	gb := mb / 1024
	if gb > 1 {
		return fmt.Sprintf("%d GB", gb)
	}
	if mb > 1 {
		return fmt.Sprintf("%d MB", mb)
	}
	if kb > 1 {
		return fmt.Sprintf("%d KB", kb)
	}
	return fmt.Sprintf("%d bytes", n)
}

func fetchBlob(ctx context.Context, target string, headers map[string]string) (io.ReadCloser, error) {
	if v := strings.TrimPrefix(target, "data:"); v != target {
		parts := strings.Split(v, ",")
		if len(parts) != 2 {
			return nil, errors.New("malformed data URL")
		}

		if strings.Contains(parts[0], ";base64") {
			output, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				return nil, fmt.Errorf("decoding base64 data: %v", err)
			}
			return ioutil.NopCloser(bytes.NewReader(output)), nil
		}
		return ioutil.NopCloser(bytes.NewReader([]byte(parts[1]))), nil
	}

	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %v", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	client := http.DefaultClient
	if c, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		client = c
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching data: %v", err)
	}
	return resp.Body, nil
}

func errorFromResponse(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusBadRequest:
		v := make(map[string]string)
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			return fmt.Errorf("bad request: parsing response body: %v", err)
		}
		if message, ok := v["message"]; ok {
			return fmt.Errorf("bad request: %v", message)
		}
	}
	return fmt.Errorf("unexpected response status: %q", resp.Status)
}
