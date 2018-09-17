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

// This binary provides an htsget server that backs onto resources in GCS.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/googlegenomics/htsget/api"
	"github.com/googlegenomics/htsget/internal/analytics"
)

var (
	port      = flag.Int("port", 80, "HTTP service port")
	blockSize = flag.Uint64("block_size", 1024*1024*1024, "block size soft limit")

	secure    = flag.Bool("secure", false, "serve in HTTPS-only mode and forward client bearer tokens")
	httpsCert = flag.String("https_cert", "", "HTTPS certificate file")
	httpsKey  = flag.String("https_key", "", "HTTPS key file")

	buckets = flag.String("buckets", "", "if set, restricts reads to a comma-separated list of buckets")

	// Enable or disable anonymous usage tracking.
	//
	// If enabled, anonymous information about requests handled by the server is
	// logged to Google via Google Analytics.
	//
	// This information helps Google determine how well the software is
	// performing and where improvements should be made.  No user identifying
	// information is ever sent to Google.
	trackUsage = flag.Bool("track_usage", false, "anonymous usage tracking")
)

func main() {
	flag.Parse()

	if *secure && (*httpsCert == "" || *httpsKey == "") {
		log.Fatalf("You must specify both -https_cert and -https_key in secure mode.")
	}

	newStorageClient := api.NewPublicClient
	if *secure {
		newStorageClient = api.NewClientFromBearerToken
	}

	server := api.NewServer(newStorageClient, *blockSize)
	server.Export(http.DefaultServeMux)

	if *buckets != "" {
		server.Whitelist(strings.Split(*buckets, ","))
	}

	handler := http.Handler(http.DefaultServeMux)
	if *trackUsage {
		log.Printf("Enabling anonymous usage tracking")

		client := analytics.NewClient("UA-103022118-1", uuid.New().String())
		handler = analytics.TrackingHandler(handler, func(hits []analytics.Hit) {
			if err := client.Send(hits); err != nil {
				log.Printf("Failed to send %d hits to analytics: %v", len(hits), err)
			}
		})
	}

	address := fmt.Sprintf(":%d", *port)
	if *secure {
		if err := http.ListenAndServeTLS(address, *httpsCert, *httpsKey, handler); err != nil {
			log.Fatalf("HTTPS server returned an error: %v", err)
		}
	} else {
		if err := http.ListenAndServe(address, handler); err != nil {
			log.Fatalf("HTTP server returned an error: %v", err)
		}
	}
}
