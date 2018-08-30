package htsget

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/googlegenomics/htsget/internal/api"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	http.HandleFunc("/", handleRequest)
}

var (
	mux              *http.ServeMux
	initializeServer sync.Once
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
	initializeServer.Do(func() {
		mux = http.NewServeMux()
		server := api.NewServer(newAppEngineClient, 8*1024*1024)
		if list := os.Getenv("BUCKET_WHITELIST"); list != "" {
			server.Whitelist(strings.Split(list, ","))
		}
		server.Export(mux)
	})
	mux.ServeHTTP(w, r)
}

func newAppEngineClient(req *http.Request) (*storage.Client, http.Header, error) {
	ctx := appengine.NewContext(req)
	gcs, err := storage.NewClient(ctx, option.WithHTTPClient(urlfetch.Client(ctx)))
	if err != nil {
		return nil, nil, fmt.Errorf("creating storage client: %v", err)
	}

	return gcs, nil, nil
}
