package file

import (
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
	x := w.Body.String()
	x = x
	pwd, _ := os.Getwd()
	pwd = pwd
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}
