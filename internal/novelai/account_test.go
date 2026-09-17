package novelai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnlasBalance(t *testing.T) {
	var (
		path          string
		authorization string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"tier":3,"isActive":true,` +
				`"trainingStepsLeft":{"fixedTrainingStepsLeft":9961,"purchasedTrainingSteps":32},` +
				`"usage":{"percent":98,"isNegative":false,"timeUntilNextPercent":7888}}`,
		))
	}))

	t.Cleanup(server.Close)

	balance, err := New(server.URL, "sk-test", false).Anlas(context.Background())
	if err != nil {
		t.Fatalf("Anlas() error: %v", err)
	}

	if path != "/user/subscription" {
		t.Errorf("path = %q, want /user/subscription", path)
	}

	if authorization != "Bearer sk-test" {
		t.Errorf("authorization = %q, want Bearer sk-test", authorization)
	}

	if balance.Total != 9993 || balance.Subscription != 9961 || balance.Purchased != 32 {
		t.Errorf("balance = %+v, want total 9993, subscription 9961, purchased 32", balance)
	}

	if balance.UsagePercent == nil || *balance.UsagePercent != 98 {
		t.Errorf("usage percent = %v, want 98", balance.UsagePercent)
	}
}

func TestAnlasWithoutUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"trainingStepsLeft":{"fixedTrainingStepsLeft":5}}`))
	}))

	t.Cleanup(server.Close)

	balance, err := New(server.URL, "k", false).Anlas(context.Background())
	if err != nil {
		t.Fatalf("Anlas() error: %v", err)
	}

	if balance.Total != 5 || balance.Purchased != 0 {
		t.Errorf("balance = %+v, want total 5 and purchased 0", balance)
	}

	if balance.UsagePercent != nil {
		t.Errorf("usage percent = %v, want nil", balance.UsagePercent)
	}
}

func TestAnlasHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Please refresh NovelAI.net."))
	}))

	t.Cleanup(server.Close)

	_, err := New(server.URL, "k", false).Anlas(context.Background())
	if err == nil {
		t.Fatal("Anlas() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want the status", err.Error())
	}
}

func TestAnlasMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))

	t.Cleanup(server.Close)

	if _, err := New(server.URL, "k", false).Anlas(context.Background()); err == nil {
		t.Fatal("Anlas() error = nil, want an error")
	}
}
