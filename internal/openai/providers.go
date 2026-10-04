package openai

import (
	"fmt"
	"net/url"
	"strings"
)

// Provider is one OpenAI-compatible endpoint and the key to call it with. An endpoint that asks for no key takes an
// empty APIKey, which drops the Authorization header.
type Provider struct {
	Name    string
	BaseURL string
	APIKey  string
}

// ParseProviders reads the configured provider list: comma-separated entries, each either name=endpoint=key or
// endpoint=key. The name defaults to the endpoint's host when it is left out, and the key may be left out for an
// endpoint that asks for none.
//
// An empty list is not an error; it means no provider is configured.
func ParseProviders(raw string) ([]Provider, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	entries := strings.Split(raw, ",")
	providers := make([]Provider, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		provider, err := parseProvider(strings.TrimSpace(entry))
		if err != nil {
			return nil, err
		}

		folded := strings.ToLower(provider.Name)
		if _, duplicate := seen[folded]; duplicate {
			return nil, fmt.Errorf("openai: %q is the name of two providers; name one of them", provider.Name)
		}

		seen[folded] = struct{}{}
		providers = append(providers, provider)
	}

	return providers, nil
}

func parseProvider(entry string) (Provider, error) {
	if entry == "" {
		return Provider{}, fmt.Errorf("openai: the provider list has an empty entry")
	}

	name, endpoint, key := splitProviderEntry(entry)

	parsed, err := parseEndpoint(endpoint)
	if err != nil {
		return Provider{}, fmt.Errorf("openai: %s: %w (want name=endpoint=key)", providerLabel(name), err)
	}

	if name == "" {
		name = parsed.Host
	}

	return Provider{
		Name:    name,
		BaseURL: strings.TrimSuffix(parsed.String(), "/"),
		APIKey:  key,
	}, nil
}

// providerLabel names a provider in an error without echoing the key the entry may carry.
func providerLabel(name string) string {
	if name == "" {
		return "the entry without a name"
	}

	return fmt.Sprintf("provider %q", name)
}

// splitProviderEntry splits an entry, whose first field is a name unless it is the endpoint itself. The key takes
// everything after the second field's first "=", so a key that carries padding survives.
func splitProviderEntry(entry string) (name, endpoint, key string) {
	first, rest, ok := strings.Cut(entry, "=")
	if !ok {
		return "", entry, ""
	}

	if isEndpoint(first) {
		return "", first, rest
	}

	endpoint, key, _ = strings.Cut(rest, "=")

	return first, endpoint, key
}

func parseEndpoint(endpoint string) (*url.URL, error) {
	if !isEndpoint(endpoint) {
		return nil, fmt.Errorf("endpoint %q is not an http or https URL", endpoint)
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint %q is not a URL: %w", endpoint, err)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("endpoint %q must include a host", endpoint)
	}

	return parsed, nil
}

func isEndpoint(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
