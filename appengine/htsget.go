package htsget

import (
	"net/http"
	"os"
	"strings"

	"github.com/googlegenomics/htsget/api"
	"google.golang.org/appengine"
)

func init() {
	mux := http.NewServeMux()
	server := api.NewServer(newAppEngineClient, 8*1024*1024)
	if list := os.Getenv("BUCKET_WHITELIST"); list != "" {
		server.Whitelist(strings.Split(list, ","))
	}
	server.Export(mux)
	http.HandleFunc("/", mux.ServeHTTP)
}

func newAppEngineClient(req *http.Request) (api.Client, http.Header, error) {
	return api.NewClientFromBearerToken(req.WithContext(appengine.NewContext(req)))
}
