package mcp

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewRegistersTools(t *testing.T) {
	srv, err := New(zapNop(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("New() returned nil server")
	}
}

func zapNop() *zap.Logger {
	return zap.NewNop()
}
