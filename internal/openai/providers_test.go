package openai

import (
	"slices"
	"strings"
	"testing"
)

func TestParseProviders(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []Provider
	}{
		{name: "empty", raw: "", want: nil},
		{name: "blank", raw: "   ", want: nil},
		{
			name: "named",
			raw:  "primary=https://api.example.com/api/v1=sk-xxx",
			want: []Provider{{Name: "primary", BaseURL: "https://api.example.com/api/v1", APIKey: "sk-xxx"}},
		},
		{
			name: "name from the host",
			raw:  "https://api.example.com/api/v1=sk-xxx",
			want: []Provider{{Name: "api.example.com", BaseURL: "https://api.example.com/api/v1", APIKey: "sk-xxx"}},
		},
		{
			name: "host with a port",
			raw:  "http://127.0.0.1:8000/v1",
			want: []Provider{{Name: "127.0.0.1:8000", BaseURL: "http://127.0.0.1:8000/v1"}},
		},
		{
			name: "no key",
			raw:  "local=http://127.0.0.1:8000/v1",
			want: []Provider{{Name: "local", BaseURL: "http://127.0.0.1:8000/v1"}},
		},
		{
			name: "trailing slash trimmed",
			raw:  "primary=https://api.example.com/api/v1/=",
			want: []Provider{{Name: "primary", BaseURL: "https://api.example.com/api/v1"}},
		},
		{
			name: "padded key kept whole",
			raw:  "primary=https://api.example.com/api/v1=sk-abc=",
			want: []Provider{{Name: "primary", BaseURL: "https://api.example.com/api/v1", APIKey: "sk-abc="}},
		},
		{
			name: "several in order",
			raw:  "primary=https://api.example.com/api/v1=sk-xxx, local=http://127.0.0.1:8000/v1=sk-local",
			want: []Provider{
				{Name: "primary", BaseURL: "https://api.example.com/api/v1", APIKey: "sk-xxx"},
				{Name: "local", BaseURL: "http://127.0.0.1:8000/v1", APIKey: "sk-local"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProviders(tt.raw)
			if err != nil {
				t.Fatalf("ParseProviders(%q) error: %v", tt.raw, err)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseProviders(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseProvidersKeepsTheKeyOutOfErrors(t *testing.T) {
	_, err := ParseProviders("primary=nonsense=sk-secret")
	if err == nil {
		t.Fatal("ParseProviders() error = nil, want an error")
	}

	if strings.Contains(err.Error(), "sk-secret") {
		t.Errorf("error = %q, want it to keep the key out", err.Error())
	}
}

func TestParseProvidersErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "no separator", raw: "sk-xxx", want: "want name=endpoint=key"},
		{name: "key where the endpoint goes", raw: "primary=sk-xxx", want: "not an http or https URL"},
		{name: "non-http scheme", raw: "primary=ftp://api.example.com=sk-xxx", want: "not an http or https URL"},
		{name: "no host", raw: "primary=https://=sk-xxx", want: "must include a host"},
		{name: "empty entry", raw: "primary=https://api.example.com/api/v1=sk-xxx,", want: "empty entry"},
		{name: "leading empty entry", raw: ",primary=https://api.example.com/api/v1=sk-xxx", want: "empty entry"},
		{
			name: "duplicate names",
			raw:  "primary=https://api.example.com/api/v1=sk-xxx,PRIMARY=https://other.example.com/v1=sk-yyy",
			want: "two providers",
		},
		{
			name: "duplicate host-derived names",
			raw:  "https://api.example.com/v1=sk-xxx,https://api.example.com/api/v1=sk-yyy",
			want: "two providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProviders(tt.raw)
			if err == nil {
				t.Fatalf("ParseProviders(%q) error = nil, want an error", tt.raw)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}
