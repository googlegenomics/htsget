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

func setupBlockRouter() *gin.Engine {
	r := gin.Default()
	r.GET("/block/:id", NewBlockHandler("./testdata"))
	return r
}

func TestBlockRoute(t *testing.T) {
	router := setupBlockRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/block/wgs_bam_NA12878_20k_b37_NA12878?start=0&end=14705", nil)
	router.ServeHTTP(w, req)
	f, err := os.Open("./testdata/head.bam")
	assert.Equal(t, nil, err)
	bam, err := ioutil.ReadAll(f)
	assert.Equal(t, nil, err)
	bamHtsget := w.Body.Bytes()
	assert.Equal(t, bam, bamHtsget)
	assert.Equal(t, 200, w.Code)
	f.Close()
}
