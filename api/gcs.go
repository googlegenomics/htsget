package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// GCSClient is Client for accessing Google Cloud Storage.
type GCSClient struct {
	*storage.Client
}

// NewObjectHandle returns a handle to a specified object in the
// storage engine.
func (c GCSClient) NewObjectHandle(bucket, object string) ObjectHandle {
	return gcsObjectHandle{c.Bucket(bucket).Object(object)}
}

type gcsObjectHandle struct {
	*storage.ObjectHandle
}

func (h gcsObjectHandle) NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return h.ObjectHandle.NewRangeReader(ctx, offset, length)
}

var (
	defaultStorageClient           *storage.Client
	initializeDefaultStorageClient sync.Once
)

func newClientWithOptions(opts ...option.ClientOption) (Client, http.Header, error) {
	initializeDefaultStorageClient.Do(func() {
		gcs, err := storage.NewClient(context.Background(), opts...)
		if err != nil {
			log.Fatalf("Creating default storage client: %v", err)
		}
		defaultStorageClient = gcs
	})
	return GCSClient{defaultStorageClient}, nil, nil
}

// NewDefaultClient returns a storage client that uses the application default
// credentials.  It caches the storage client for efficiency.
func NewDefaultClient(_ *http.Request) (Client, http.Header, error) {
	return newClientWithOptions()
}

// NewPublicClient returns a storage client that does not use any form of
// client authorization.  It can only be used to read publicly-readable
// objects. It caches the storage client for efficiency.
func NewPublicClient(_ *http.Request) (Client, http.Header, error) {
	return newClientWithOptions(option.WithHTTPClient(http.DefaultClient))
}

// NewClientFromBearerToken constructs a storage client that uses the OAuth2
// bearer token found in req to make storage requests.  It returns the
// authorization header containing the bearer token as well to allow subsequent
// requests to be authenticated correctly.
func NewClientFromBearerToken(req *http.Request) (Client, http.Header, error) {
	authorization := req.Header.Get("Authorization")

	fields := strings.Split(authorization, " ")
	if len(fields) != 2 || fields[0] != "Bearer" {
		return nil, nil, errMissingOrInvalidToken
	}

	token := oauth2.Token{
		TokenType:   fields[0],
		AccessToken: fields[1],
	}
	client, err := storage.NewClient(req.Context(), option.WithTokenSource(oauth2.StaticTokenSource(&token)))
	if err != nil {
		return nil, nil, fmt.Errorf("creating client with token source: %v", err)
	}

	return GCSClient{client}, map[string][]string{
		"Authorization": []string{authorization},
	}, nil
}

func newStorageError(context string, err error) error {
	if err == errMissingOrInvalidToken {
		return newPermissionDeniedError(context, err)
	}
	if err == storage.ErrObjectNotExist {
		return newNotFoundError("object does not exist", err)
	}
	if err, ok := err.(*googleapi.Error); ok {
		switch err.Code {
		case http.StatusUnauthorized:
			return newInvalidAuthenticationError(context, err)
		case http.StatusForbidden:
			return newPermissionDeniedError(context, err)
		}
	}
	return err
}
