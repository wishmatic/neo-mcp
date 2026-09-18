package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func exampleMeta(model, url string) ExampleMeta {
	return ExampleMeta{
		Model: model,
		Tool:  "txt2img",
		Query: `{"model":"` + model + `","prompt":"a cat"}`,
		URL:   url,
	}
}

func TestSaveExampleRoundTrip(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	meta := exampleMeta("nai-diffusion-5-full", "https://cdn.example.com/a.png")

	id, err := client.SaveExample(ctx, meta, 16)
	if err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	if id <= 0 {
		t.Fatalf("id = %d, want a positive id", id)
	}

	examples, err := client.RandomExamples(ctx, meta.Model, 1)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 {
		t.Fatalf("examples = %d, want 1", len(examples))
	}

	got := examples[0]
	if got.ID != id || got.Model != meta.Model || got.Tool != meta.Tool || got.Query != meta.Query || got.URL != meta.URL {
		t.Fatalf("example = %+v, want %+v", got, meta)
	}

	if delta := time.Since(got.CreatedAt); delta < 0 || delta > time.Minute {
		t.Errorf("CreatedAt = %s, want a recent timestamp", got.CreatedAt)
	}
}

func TestSaveExampleRetainsNewest(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	var ids []int64

	for i := range 17 {
		id, err := client.SaveExample(ctx, exampleMeta("m", fmt.Sprintf("https://cdn.example.com/%d.png", i)), 16)
		if err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}

		ids = append(ids, id)
	}

	examples, err := client.RandomExamples(ctx, "m", 100)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 16 {
		t.Fatalf("examples = %d, want 16", len(examples))
	}

	kept := make(map[int64]bool, len(examples))
	for _, example := range examples {
		kept[example.ID] = true
	}

	if kept[ids[0]] {
		t.Error("the oldest example was kept")
	}

	for _, id := range ids[1:] {
		if !kept[id] {
			t.Errorf("example %d was pruned", id)
		}
	}
}

func TestSaveExampleRetentionIsPerModel(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.SaveExample(ctx, exampleMeta("a", "https://cdn.example.com/a1.png"), 2); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	for i := range 5 {
		url := fmt.Sprintf("https://cdn.example.com/b%d.png", i)

		if _, err := client.SaveExample(ctx, exampleMeta("b", url), 2); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}
	}

	a, err := client.RandomExamples(ctx, "a", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(a) != 1 {
		t.Fatalf("examples for a = %d, want 1", len(a))
	}

	b, err := client.RandomExamples(ctx, "b", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(b) != 2 {
		t.Fatalf("examples for b = %d, want 2", len(b))
	}
}

func TestSaveExampleCountNeverExceedsRetention(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for i := range 20 {
		url := fmt.Sprintf("https://cdn.example.com/%d.png", i)

		if _, err := client.SaveExample(ctx, exampleMeta("m", url), 5); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}

		examples, err := client.RandomExamples(ctx, "m", 100)
		if err != nil {
			t.Fatalf("RandomExamples() error: %v", err)
		}

		if len(examples) > 5 {
			t.Fatalf("after save %d: examples = %d, want at most 5", i, len(examples))
		}
	}
}

func TestSaveExampleValidation(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	valid := exampleMeta("m", "https://cdn.example.com/a.png")

	tests := []struct {
		name   string
		mutate func(*ExampleMeta)
		retain int
	}{
		{name: "empty model", mutate: func(m *ExampleMeta) { m.Model = "" }, retain: 1},
		{name: "whitespace model", mutate: func(m *ExampleMeta) { m.Model = "  " }, retain: 1},
		{name: "empty tool", mutate: func(m *ExampleMeta) { m.Tool = "" }, retain: 1},
		{name: "empty query", mutate: func(m *ExampleMeta) { m.Query = "" }, retain: 1},
		{name: "invalid query", mutate: func(m *ExampleMeta) { m.Query = "not json" }, retain: 1},
		{name: "empty url", mutate: func(m *ExampleMeta) { m.URL = "" }, retain: 1},
		{name: "retention zero", mutate: func(*ExampleMeta) {}, retain: 0},
		{name: "retention negative", mutate: func(*ExampleMeta) {}, retain: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := valid
			tt.mutate(&meta)

			if _, err := client.SaveExample(ctx, meta, tt.retain); err == nil {
				t.Fatal("SaveExample() error = nil, want an error")
			}
		})
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want nothing written", len(examples))
	}
}

func TestSaveExampleTrimsModelToolAndURL(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	meta := ExampleMeta{
		Model: "  m  ",
		Tool:  "  txt2img  ",
		Query: `{"prompt":"a cat"}`,
		URL:   "  https://cdn.example.com/a.png  ",
	}

	if _, err := client.SaveExample(ctx, meta, 1); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	examples, err := client.RandomExamples(ctx, "m", 1)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 {
		t.Fatalf("examples = %d, want 1", len(examples))
	}

	got := examples[0]
	if got.Model != "m" || got.Tool != "txt2img" || got.URL != "https://cdn.example.com/a.png" {
		t.Fatalf("example = %+v, want trimmed values", got)
	}
}

func TestSaveExamplePreservesQueryAndURL(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	query := "{\n  \"prompt\": \"a cat | dog\",\n  \"negative_prompt\": \"blurry\\nbad\",\n  \"note\": \"multi-byte: 猫\"\n}"
	url := "https://cdn.example.com/i/mcp/a b.png?X-Amz-Signature=abc&x=1#frag"

	if _, err := client.SaveExample(ctx, ExampleMeta{Model: "m", Tool: "img2img", Query: query, URL: url}, 1); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	examples, err := client.RandomExamples(ctx, "m", 1)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if examples[0].Query != query {
		t.Errorf("query = %q, want %q", examples[0].Query, query)
	}

	if examples[0].URL != url {
		t.Errorf("url = %q, want %q", examples[0].URL, url)
	}

	if !json.Valid([]byte(examples[0].Query)) {
		t.Error("stored query is no longer valid JSON")
	}

	longURL := "https://cdn.example.com/" + strings.Repeat("a", 2000) + ".png"

	meta := ExampleMeta{Model: "m", Tool: "txt2img", Query: query, URL: longURL}

	if _, err := client.SaveExample(ctx, meta, 2); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	examples, err = client.RandomExamples(ctx, "m", 2)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	found := false

	for _, example := range examples {
		if example.URL == longURL {
			found = true
		}
	}

	if !found {
		t.Error("a 2000 character URL did not round-trip")
	}
}

func TestRandomExamples(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for i := range 16 {
		url := fmt.Sprintf("https://cdn.example.com/%d.png", i)

		if _, err := client.SaveExample(ctx, exampleMeta("m", url), 16); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}
	}

	if _, err := client.SaveExample(ctx, exampleMeta("other", "https://cdn.example.com/other.png"), 16); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	distinct := make(map[string]bool)

	for range 50 {
		examples, err := client.RandomExamples(ctx, "m", 2)
		if err != nil {
			t.Fatalf("RandomExamples() error: %v", err)
		}

		if len(examples) != 2 {
			t.Fatalf("examples = %d, want 2", len(examples))
		}

		ids := make([]string, 0, 2)

		for _, example := range examples {
			if example.Model != "m" {
				t.Fatalf("example model = %q, want m", example.Model)
			}

			ids = append(ids, fmt.Sprint(example.ID))
		}

		if ids[0] == ids[1] {
			t.Fatalf("duplicate ids in one result: %v", ids)
		}

		distinct[ids[0]+","+ids[1]] = true
	}

	if len(distinct) < 2 {
		t.Errorf("50 draws returned %d distinct result sets, want random selection", len(distinct))
	}
}

func TestRandomExamplesFewerThanRequested(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	empty, err := client.RandomExamples(ctx, "m", 2)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatalf("examples = %v, want an empty slice", empty)
	}

	if _, err := client.SaveExample(ctx, exampleMeta("m", "https://cdn.example.com/a.png"), 1); err != nil {
		t.Fatalf("SaveExample() error: %v", err)
	}

	one, err := client.RandomExamples(ctx, "m", 2)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(one) != 1 {
		t.Fatalf("examples = %d, want 1", len(one))
	}
}

func TestRandomExamplesValidation(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.RandomExamples(ctx, "", 1); err == nil {
		t.Error("RandomExamples() with an empty model error = nil, want an error")
	}

	if _, err := client.RandomExamples(ctx, "m", 0); err == nil {
		t.Error("RandomExamples() with count 0 error = nil, want an error")
	}

	if _, err := client.RandomExamplesByNSFW(ctx, "", 1, true); err == nil {
		t.Error("RandomExamplesByNSFW() with an empty model error = nil, want an error")
	}

	if _, err := client.RandomExamplesByNSFW(ctx, "m", 0, true); err == nil {
		t.Error("RandomExamplesByNSFW() with count 0 error = nil, want an error")
	}
}

func TestSaveExamplesWritesEachURL(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	meta := exampleMeta("m", "ignored")

	urls := []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"}
	if err := client.SaveExamples(ctx, meta, urls, 16); err != nil {
		t.Fatalf("SaveExamples() error: %v", err)
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 2 {
		t.Fatalf("examples = %d, want 2", len(examples))
	}

	stored := make(map[string]bool, len(examples))
	for _, example := range examples {
		if example.Model != "m" || example.Tool != "txt2img" || example.Query != meta.Query {
			t.Errorf("example = %+v, want the shared metadata", example)
		}

		stored[example.URL] = true
	}

	for _, url := range urls {
		if !stored[url] {
			t.Errorf("url %q was not saved", url)
		}
	}
}

func TestSaveExamplesEmptyURLsIsNoOp(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if err := client.SaveExamples(ctx, exampleMeta("m", ""), nil, 4); err != nil {
		t.Fatalf("SaveExamples() error: %v", err)
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want none", len(examples))
	}
}

func TestSaveExamplesRetention(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	urls := []string{
		"https://cdn.example.com/1.png",
		"https://cdn.example.com/2.png",
		"https://cdn.example.com/3.png",
	}

	if err := client.SaveExamples(ctx, exampleMeta("m", ""), urls, 2); err != nil {
		t.Fatalf("SaveExamples() error: %v", err)
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 2 {
		t.Fatalf("examples = %d, want the newest 2", len(examples))
	}

	for _, example := range examples {
		if example.URL == urls[0] {
			t.Error("the oldest example was kept")
		}
	}
}

func TestSaveExamplesInvalidURLStillWritesValid(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	urls := []string{"", "https://cdn.example.com/a.png"}
	if err := client.SaveExamples(ctx, exampleMeta("m", ""), urls, 4); err == nil {
		t.Fatal("SaveExamples() error = nil, want the invalid URL error")
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 1 || examples[0].URL != "https://cdn.example.com/a.png" {
		t.Fatalf("examples = %+v, want the valid URL written", examples)
	}
}

func TestSaveExamplesInvalidRetention(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	err := client.SaveExamples(ctx, exampleMeta("m", ""), []string{"https://cdn.example.com/a.png"}, 0)
	if err == nil {
		t.Fatal("SaveExamples() error = nil, want an error")
	}

	examples, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(examples) != 0 {
		t.Fatalf("examples = %d, want nothing written", len(examples))
	}
}

func TestSaveExampleConcurrent(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	const (
		writers   = 8
		perWriter = 3
	)

	models := []string{"a", "b"}

	var (
		wg   sync.WaitGroup
		errs = make(chan error, writers*perWriter)
	)

	for writer := range writers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			model := models[writer%len(models)]

			for i := range perWriter {
				url := fmt.Sprintf("https://cdn.example.com/%s-%d-%d.png", model, writer, i)
				if _, err := client.SaveExample(ctx, exampleMeta(model, url), 4); err != nil {
					errs <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent SaveExample() error: %v", err)
	}

	for _, model := range models {
		examples, err := client.RandomExamples(ctx, model, 100)
		if err != nil {
			t.Fatalf("RandomExamples() error: %v", err)
		}

		if len(examples) > 4 {
			t.Fatalf("model %s has %d examples, want at most 4", model, len(examples))
		}
	}
}

func TestExampleNSFWRoundTripAndFilter(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	for i, nsfw := range []bool{false, false, true} {
		meta := exampleMeta("m", fmt.Sprintf("https://cdn.example.com/%d.png", i))
		meta.NSFW = nsfw

		if _, err := client.SaveExample(ctx, meta, 16); err != nil {
			t.Fatalf("SaveExample() error: %v", err)
		}
	}

	all, err := client.RandomExamples(ctx, "m", 10)
	if err != nil {
		t.Fatalf("RandomExamples() error: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("examples = %d, want 3", len(all))
	}

	nsfw, err := client.RandomExamplesByNSFW(ctx, "m", 10, true)
	if err != nil {
		t.Fatalf("RandomExamplesByNSFW(true) error: %v", err)
	}

	if len(nsfw) != 1 || !nsfw[0].NSFW {
		t.Fatalf("nsfw examples = %+v, want exactly the NSFW one", nsfw)
	}

	clean, err := client.RandomExamplesByNSFW(ctx, "m", 10, false)
	if err != nil {
		t.Fatalf("RandomExamplesByNSFW(false) error: %v", err)
	}

	if len(clean) != 2 {
		t.Fatalf("clean examples = %d, want 2", len(clean))
	}

	for _, example := range clean {
		if example.NSFW {
			t.Errorf("example = %+v, want non-NSFW", example)
		}
	}
}
