package main

import (
	"flag"

	"github.com/gin-gonic/gin"
	"github.com/googlegenomics/htsget/htsget-multisource-server/file"
)

//TODO make this an environment variable or something
const defaultBaseUrl = "http://localhost:8080"

var (
	port      = flag.Int("port", 8080, "HTTP service port")
	blockSize = flag.Uint64("block_size", 1024*1024*1024, "block size soft limit")

	secure    = flag.Bool("secure", false, "serve in HTTPS-only mode and forward client bearer tokens")
	httpsCert = flag.String("https_cert", "", "HTTPS certificate file")
	httpsKey  = flag.String("https_key", "", "HTTPS key file")

	baseURL = flag.String("base_url", defaultBaseUrl, "HTTPS key file")

	azureBuckets = flag.String("azure-buckets", "", "if set, restricts reads to a comma-separated list of buckets")
	directory    = flag.String("directory", "", "directory that contains bam/bai files")

	// Enable or disable anonymous usage tracking.
	//
	// If enabled, anonymous information about requests handled by the server is
	// logged to Google via Google Analytics.
	//
	// This information helps Google determine how well the software is
	// performing and where improvements should be made.  No user identifying
	// information is ever sent to Google.
	// trackUsage = flag.Bool("track_usage", false, "anonymous usage tracking")
)

func main() {
	flag.Parse()
	router := gin.Default()
	// defer profile.Start(profile.MemProfile).Stop()

	var blockHandler func(c *gin.Context)
	var readsHandler func(c *gin.Context)

	if *directory != "" {
		blockHandler = file.NewBlockHandler(*directory)
		readsHandler = file.NewReadsHandler(*directory, *blockSize, *baseURL)
	} else if *azureBuckets != "" {

	} else {
		panic("no directory or buckets specified")
	}

	router.GET("/block/:id", blockHandler)
	router.GET("/reads/:id", readsHandler)
	router.Run()
}
