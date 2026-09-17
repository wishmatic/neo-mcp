package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/neo-mcp/internal/store"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func newReviewStore(t *testing.T) *store.Client {
	t.Helper()

	client, err := store.New(filepath.Join(t.TempDir(), "neo.db"))
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestReviewToolsRegistration(t *testing.T) {
	withStore, err := New(Deps{Log: zapNop(), Store: newReviewStore(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	registered := toolNames(t, withStore)

	for _, tool := range []string{"add_review", "get_reviews", "get_review", "delete_review"} {
		if !slices.Contains(registered, tool) {
			t.Errorf("tools = %v, want %s", registered, tool)
		}
	}

	withoutStore, err := New(Deps{Log: zapNop()})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	absent := toolNames(t, withoutStore)

	for _, tool := range []string{"add_review", "get_reviews", "get_review", "delete_review"} {
		if slices.Contains(absent, tool) {
			t.Errorf("tools = %v, want no %s without a store", absent, tool)
		}
	}
}

func TestAddReviewSchema(t *testing.T) {
	s := addReviewSchema()

	for _, field := range []string{"model", "rating", "comment"} {
		prop, ok := s.Properties[field]
		if !ok {
			t.Fatalf("%s property is missing", field)
		}

		if !slices.Contains(s.Required, field) {
			t.Errorf("%s is not required", field)
		}

		if prop.Default != nil {
			t.Errorf("%s must not have a default", field)
		}
	}

	if min := s.Properties["rating"].Minimum; min == nil || *min != 1 {
		t.Errorf("rating minimum = %v, want 1", min)
	}

	if max := s.Properties["rating"].Maximum; max == nil || *max != 10 {
		t.Errorf("rating maximum = %v, want 10", max)
	}

	if limit := s.Properties["model"].MaxLength; limit == nil || *limit != store.MaxModelLength {
		t.Errorf("model maxLength = %v, want %d", limit, store.MaxModelLength)
	}

	if limit := s.Properties["comment"].MaxLength; limit == nil || *limit != store.MaxCommentLength {
		t.Errorf("comment maxLength = %v, want %d", limit, store.MaxCommentLength)
	}

	for _, field := range []string{"prompt", "image_url", "agent_comment"} {
		prop, ok := s.Properties[field]
		if !ok {
			t.Fatalf("%s property is missing", field)
		}

		if slices.Contains(s.Required, field) {
			t.Errorf("%s must not be required", field)
		}

		if prop.Default != nil {
			t.Errorf("%s must not have a default", field)
		}
	}

	if limit := s.Properties["prompt"].MaxLength; limit == nil || *limit != store.MaxPromptLength {
		t.Errorf("prompt maxLength = %v, want %d", limit, store.MaxPromptLength)
	}

	if limit := s.Properties["agent_comment"].MaxLength; limit == nil || *limit != store.MaxAgentCommentLength {
		t.Errorf("agent_comment maxLength = %v, want %d", limit, store.MaxAgentCommentLength)
	}

	imageURL := s.Properties["image_url"]

	if limit := imageURL.MaxLength; limit == nil || *limit != store.MaxImageURLLength {
		t.Errorf("image_url maxLength = %v, want %d", limit, store.MaxImageURLLength)
	}

	if imageURL.Format != "uri" {
		t.Errorf("image_url format = %q, want uri", imageURL.Format)
	}
}

func TestGetReviewsSchema(t *testing.T) {
	s := getReviewsSchema()

	if slices.Contains(s.Required, "model") {
		t.Error("model must not be required")
	}

	if limit := s.Properties["model"].MaxLength; limit == nil || *limit != store.MaxModelLength {
		t.Errorf("model maxLength = %v, want %d", limit, store.MaxModelLength)
	}
}

func TestGetReviewSchema(t *testing.T) {
	s := getReviewSchema()

	if !slices.Contains(s.Required, "id") {
		t.Error("id is not required")
	}

	if s.Properties["id"].Default != nil {
		t.Error("id must not have a default")
	}
}

func TestDeleteReviewSchema(t *testing.T) {
	s := deleteReviewSchema()

	if !slices.Contains(s.Required, "id") {
		t.Error("id is not required")
	}

	if s.Properties["id"].Default != nil {
		t.Error("id must not have a default")
	}
}

func TestAddReviewInvalidRating(t *testing.T) {
	h := &handlers{log: zapNop(), store: newReviewStore(t)}

	for _, rating := range []int{0, 11} {
		_, _, err := h.addReview(context.Background(), nil, addReviewInput{Model: "m", Rating: rating, Comment: "no"})
		if err == nil {
			t.Fatalf("addReview(rating=%d) error = nil, want an error", rating)
		}

		if !strings.HasPrefix(err.Error(), "add_review:") {
			t.Errorf("error = %q, want an add_review prefix", err.Error())
		}
	}

	_, out, err := h.getReviews(context.Background(), nil, getReviewsInput{})
	if err != nil {
		t.Fatalf("getReviews() error: %v", err)
	}

	if out.Count != 0 {
		t.Fatalf("count = %d, want no reviews written", out.Count)
	}
}

func TestDeleteReviewUnknownID(t *testing.T) {
	h := &handlers{log: zapNop(), store: newReviewStore(t)}

	_, _, err := h.deleteReview(context.Background(), nil, deleteReviewInput{ID: 42})
	if err == nil {
		t.Fatal("deleteReview() error = nil, want an error")
	}

	if !strings.HasPrefix(err.Error(), "delete_review:") {
		t.Errorf("error = %q, want a delete_review prefix", err.Error())
	}

	if !strings.Contains(err.Error(), "42") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to name the id and the failure", err.Error())
	}
}

func TestGetReviewUnknownID(t *testing.T) {
	h := &handlers{log: zapNop(), store: newReviewStore(t)}

	_, _, err := h.getReview(context.Background(), nil, getReviewInput{ID: 42})
	if err == nil {
		t.Fatal("getReview() error = nil, want an error")
	}

	if !strings.HasPrefix(err.Error(), "get_review:") {
		t.Errorf("error = %q, want a get_review prefix", err.Error())
	}

	if !strings.Contains(err.Error(), "42") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to name the id and the failure", err.Error())
	}
}

func TestReviewToolsLogToolAndAffectedID(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	h := &handlers{log: zap.New(core), store: newReviewStore(t)}
	ctx := context.Background()

	_, added, err := h.addReview(ctx, nil, addReviewInput{Model: "m", Rating: 5, Comment: "ok"})
	if err != nil {
		t.Fatalf("addReview() error: %v", err)
	}

	if _, _, err := h.getReviews(ctx, nil, getReviewsInput{}); err != nil {
		t.Fatalf("getReviews() error: %v", err)
	}

	if _, _, err := h.getReview(ctx, nil, getReviewInput{ID: added.ID}); err != nil {
		t.Fatalf("getReview() error: %v", err)
	}

	if _, _, err := h.deleteReview(ctx, nil, deleteReviewInput{ID: added.ID}); err != nil {
		t.Fatalf("deleteReview() error: %v", err)
	}

	called := make(map[string]bool)
	ids := make(map[string]bool)

	for _, entry := range logs.All() {
		if entry.Message == "tool called" {
			if tool, ok := entry.ContextMap()["tool"].(string); ok {
				called[tool] = true
			}
		}

		if entry.Message == "review added" || entry.Message == "review deleted" {
			ids[fmt.Sprint(entry.ContextMap()["id"])] = true
		}
	}

	for _, tool := range []string{"add_review", "get_reviews", "get_review", "delete_review"} {
		if !called[tool] {
			t.Errorf("no \"tool called\" entry for %s", tool)
		}
	}

	if !ids[fmt.Sprint(added.ID)] {
		t.Errorf("no info entry naming the affected review %d", added.ID)
	}
}

func TestReviewToolsStoreError(t *testing.T) {
	client := newReviewStore(t)

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	h := &handlers{log: zapNop(), store: client}

	if _, _, err := h.getReviews(context.Background(), nil, getReviewsInput{}); err == nil {
		t.Error("getReviews() error = nil, want a store error")
	} else if !strings.HasPrefix(err.Error(), "get_reviews:") {
		t.Errorf("error = %q, want a get_reviews prefix", err.Error())
	}

	if _, _, err := h.getReview(context.Background(), nil, getReviewInput{ID: 1}); err == nil {
		t.Error("getReview() error = nil, want a store error")
	} else if !strings.HasPrefix(err.Error(), "get_review:") {
		t.Errorf("error = %q, want a get_review prefix", err.Error())
	}

	in := addReviewInput{Model: "m", Rating: 5, Comment: "ok"}

	if _, _, err := h.addReview(context.Background(), nil, in); err == nil {
		t.Error("addReview() error = nil, want a store error")
	}

	if _, _, err := h.deleteReview(context.Background(), nil, deleteReviewInput{ID: 1}); err == nil {
		t.Error("deleteReview() error = nil, want a store error")
	}
}

func TestReviewToolsEndToEnd(t *testing.T) {
	srv, err := New(Deps{Log: zapNop(), Store: newReviewStore(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	session := connectSession(t, srv)
	ctx := context.Background()

	for _, review := range []struct {
		model   string
		rating  int
		comment string
	}{
		{model: "nai-diffusion-5-full", rating: 9, comment: "Superb hands"},
		{model: "sd_xl_base_1.0.safetensors", rating: 4, comment: "Muddy at distance"},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "add_review",
			Arguments: map[string]any{
				"model":   review.model,
				"rating":  review.rating,
				"comment": review.comment,
			},
		})
		if err != nil {
			t.Fatalf("CallTool(add_review) error: %v", err)
		}

		if result.IsError {
			t.Fatalf("CallTool(add_review) tool error: %+v", result.Content)
		}

		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
		}

		if structured["model"] != review.model || structured["rating"] != float64(review.rating) {
			t.Errorf("structured content = %+v, want the added review", structured)
		}

		if id, ok := structured["id"].(float64); !ok || id <= 0 {
			t.Errorf("structured id = %v, want a positive number", structured["id"])
		}

		if !strings.Contains(textContent(t, result), "Added review") {
			t.Errorf("text = %q, want a confirmation", textContent(t, result))
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_reviews"})
	if err != nil {
		t.Fatalf("CallTool(get_reviews) error: %v", err)
	}

	if result.IsError {
		t.Fatalf("CallTool(get_reviews) tool error: %+v", result.Content)
	}

	listing := textContent(t, result)

	for _, want := range []string{
		"# Reviews",
		"Average: 6.5/10 (2 reviews)",
		"## nai-diffusion-5-full",
		"Average: 9.0/10 (1 review)",
		"## sd_xl_base_1.0.safetensors",
		"Average: 4.0/10 (1 review)",
	} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing =\n%s\nwant it to contain %q", listing, want)
		}
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
	}

	if structured["count"] != float64(2) || structured["average"] != 6.5 {
		t.Errorf("structured content = %+v, want count 2 and average 6.5", structured)
	}

	added, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add_review",
		Arguments: map[string]any{"model": "m", "rating": 1, "comment": "temporary"},
	})
	if err != nil {
		t.Fatalf("CallTool(add_review) error: %v", err)
	}

	addedContent, ok := added.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", added.StructuredContent)
	}

	id := int64(addedContent["id"].(float64))

	deleted, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_review",
		Arguments: map[string]any{"id": id},
	})
	if err != nil {
		t.Fatalf("CallTool(delete_review) error: %v", err)
	}

	if deleted.IsError {
		t.Fatalf("CallTool(delete_review) tool error: %+v", deleted.Content)
	}

	deletedContent, ok := deleted.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", deleted.StructuredContent)
	}

	if deletedContent["model"] != "m" || deletedContent["comment"] != "temporary" {
		t.Errorf("structured content = %+v, want the deleted review", deletedContent)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_reviews"})
	if err != nil {
		t.Fatalf("CallTool(get_reviews) error: %v", err)
	}

	if strings.Contains(textContent(t, result), "temporary") {
		t.Error("the deleted review is still listed")
	}
}

func TestReviewDetailsEndToEnd(t *testing.T) {
	srv, err := New(Deps{Log: zapNop(), Store: newReviewStore(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	session := connectSession(t, srv)
	ctx := context.Background()

	added, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "add_review",
		Arguments: map[string]any{
			"model":         "nai-diffusion-5-full",
			"rating":        9,
			"comment":       "Superb hands",
			"prompt":        "a cat, cinematic lighting",
			"image_url":     "https://cdn.example.com/a.png",
			"agent_comment": "Liked the composition",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(add_review) error: %v", err)
	}

	if added.IsError {
		t.Fatalf("CallTool(add_review) tool error: %+v", added.Content)
	}

	content, ok := added.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", added.StructuredContent)
	}

	if content["prompt"] != "a cat, cinematic lighting" ||
		content["image_url"] != "https://cdn.example.com/a.png" ||
		content["agent_comment"] != "Liked the composition" {
		t.Errorf("structured content = %+v, want the recorded details", content)
	}

	id := int64(content["id"].(float64))

	listing, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_reviews"})
	if err != nil {
		t.Fatalf("CallTool(get_reviews) error: %v", err)
	}

	listingText := textContent(t, listing)

	if strings.Contains(listingText, "a cat, cinematic lighting") {
		t.Error("get_reviews leaked the prompt")
	}

	for _, want := range []string{"https://cdn.example.com/a.png", "Liked the composition"} {
		if !strings.Contains(listingText, want) {
			t.Errorf("listing =\n%s\nwant it to contain %q", listingText, want)
		}
	}

	full, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_review",
		Arguments: map[string]any{"id": id},
	})
	if err != nil {
		t.Fatalf("CallTool(get_review) error: %v", err)
	}

	if full.IsError {
		t.Fatalf("CallTool(get_review) tool error: %+v", full.Content)
	}

	fullContent, ok := full.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", full.StructuredContent)
	}

	if fullContent["prompt"] != "a cat, cinematic lighting" ||
		fullContent["image_url"] != "https://cdn.example.com/a.png" ||
		fullContent["agent_comment"] != "Liked the composition" {
		t.Errorf("structured content = %+v, want every recorded detail", fullContent)
	}

	if createdAt, ok := fullContent["created_at"].(string); !ok || createdAt == "" {
		t.Errorf("created_at = %v, want a timestamp", fullContent["created_at"])
	}

	if !strings.Contains(textContent(t, full), "a cat, cinematic lighting") {
		t.Error("get_review text omitted the prompt")
	}
}

func TestGetReviewsFiltersByModel(t *testing.T) {
	srv, err := New(Deps{Log: zapNop(), Store: newReviewStore(t)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	session := connectSession(t, srv)
	ctx := context.Background()

	for _, review := range []struct{ model, comment string }{
		{model: "a", comment: "a-note"},
		{model: "b", comment: "b-note"},
	} {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "add_review",
			Arguments: map[string]any{"model": review.model, "rating": 5, "comment": review.comment},
		}); err != nil {
			t.Fatalf("CallTool(add_review) error: %v", err)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_reviews",
		Arguments: map[string]any{"model": "a"},
	})
	if err != nil {
		t.Fatalf("CallTool(get_reviews) error: %v", err)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want map[string]any", result.StructuredContent)
	}

	if structured["model"] != "a" || structured["count"] != float64(1) || structured["average"] != 5.0 {
		t.Errorf("structured content = %+v, want only model a", structured)
	}

	listing := textContent(t, result)

	for _, want := range []string{"# Reviews for a", "a-note"} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing =\n%s\nwant it to contain %q", listing, want)
		}
	}

	if strings.Contains(listing, "b-note") {
		t.Error("the listing includes another model's review")
	}
}

func textContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("content = %d, want 1", len(result.Content))
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}

	return text.Text
}
