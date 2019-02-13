package file

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupReadsRouter() *gin.Engine {
	r := gin.Default()
	r.GET("/reads/:id", NewReadsHandler("./testdata", 1024*1024*1024, "http://yolo:8080"))
	return r
}

func TestReadRoute(t *testing.T) {
	router := setupReadsRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reads/wgs_bam_NA12878_20k_b37_NA12878?referenceName=X&start=34568064&end=72893114", nil)
	router.ServeHTTP(w, req)
	f, err := os.Open("./testdata/htsget.json")
	assert.Equal(t, nil, err)
	bam, err := ioutil.ReadAll(f)
	assert.Equal(t, nil, err)
	assert.Equal(t, bam, w.Body.Bytes())
	assert.Equal(t, 200, w.Code)
	f.Close()
}
