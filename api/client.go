package api

import (
	"context"
	"io"
)

// Client is an interface to the storage engine.
type Client interface {
	// NewObjectHandle returns a handle to a specified object in
	// the storage engine.
	NewObjectHandle(bucket, object string) ObjectHandle
}

// ObjectHandle is an interface to the actual storage engine in use.
type ObjectHandle interface {
	// NewRangeReader returns a reader that reads from a specified
	// range. Length of -1 means to capture everything until the
	// end.
	NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error)
}
