package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/wishmatic/neo-mcp/internal/imgfmt"
	"github.com/wishmatic/neo-mcp/internal/publish"
)

type uploadCapture struct {
	contentType string
	nsfw        bool
}

type fakeStore struct {
	mu      sync.Mutex
	uploads []uploadCapture
	err     error
}

func (f *fakeStore) UploadFile(_ context.Context, _ []byte, contentType string, nsfw bool) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.uploads = append(f.uploads, uploadCapture{contentType: contentType, nsfw: nsfw})

	return fmt.Sprintf("https://cdn.example.com/i/%d.%s", len(f.uploads), imgfmt.ExtensionForMediaType(contentType)), nil
}

func newFakeStore(t *testing.T) publish.Store {
	t.Helper()

	return &fakeStore{}
}

func newFailingStore(t *testing.T) publish.Store {
	t.Helper()

	return &fakeStore{err: errors.New("upload failed")}
}

func newTestPublisher(t *testing.T) *publish.Publisher {
	t.Helper()

	return publish.New(newFakeStore(t), zapNop())
}
