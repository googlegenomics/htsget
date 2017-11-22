package htsget

import (
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/googlegenomics/htsget/internal/api"
	"google.golang.org/appengine"
)

func init() {
	mux := http.NewServeMux()
	api.NewServer(newAppEngineClient, 16*1024*1024).Export(mux)
	http.HandleFunc("/", mux.ServeHTTP)
}

func newAppEngineClient(req *http.Request) (*storage.Client, http.Header, error) {
	return api.NewClientFromBearerToken(req.WithContext(appengine.NewContext(req)))
}
