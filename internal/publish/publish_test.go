package publish

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type capture struct {
	contentType string
	nsfw        bool
}

type fakeStore struct {
	mu      sync.Mutex
	capture []capture
	err     error
}

func (f *fakeStore) UploadFile(_ context.Context, _ []byte, contentType string, nsfw bool) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.capture = append(f.capture, capture{contentType: contentType, nsfw: nsfw})

	return fmt.Sprintf("https://files.example/i/%d.png", len(f.capture)), nil
}

func TestEnabled(t *testing.T) {
	if New(nil, zap.NewNop()).Enabled() {
		t.Error("Enabled() = true without a store, want false")
	}

	if !New(&fakeStore{}, zap.NewNop()).Enabled() {
		t.Error("Enabled() = false with a store, want true")
	}
}

func TestImagesKeepsOrderAndCount(t *testing.T) {
	store := &fakeStore{}
	p := New(store, zap.NewNop())

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a"), []byte("b"), []byte("c")}, "image/png", false)
	if err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(urls) != 3 || len(store.capture) != 3 {
		t.Fatalf("urls = %v, uploads = %+v, want three of each", urls, store.capture)
	}

	if urls[0] == urls[1] || urls[1] == urls[2] {
		t.Errorf("urls = %v, want distinct URLs", urls)
	}
}

func TestImagesPassesContentTypeAndNSFW(t *testing.T) {
	store := &fakeStore{}
	p := New(store, zap.NewNop())

	if _, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a")}, "image/jpeg", true); err != nil {
		t.Fatalf("Images() error: %v", err)
	}

	if len(store.capture) != 1 {
		t.Fatalf("uploads = %d, want 1", len(store.capture))
	}

	if store.capture[0].contentType != "image/jpeg" || !store.capture[0].nsfw {
		t.Errorf("upload = %+v, want image/jpeg and nsfw", store.capture[0])
	}
}

func TestImagesUploadFailure(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	p := New(&fakeStore{err: errors.New("boom")}, zap.New(core))

	urls, err := p.Images(context.Background(), "txt2img", [][]byte{[]byte("a")}, "image/png", false)
	if err == nil {
		t.Fatal("Images() error = nil, want the upload failure")
	}

	if urls != nil {
		t.Errorf("urls = %v, want none on failure", urls)
	}

	if count := logs.FilterMessageSnippet("upload failed").Len(); count != 1 {
		t.Errorf("errors = %d, want one upload failure", count)
	}
}
