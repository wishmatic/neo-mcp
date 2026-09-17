package mcp

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/wishmatic/neo-mcp/internal/store"
)

func addReviewSchema() *jsonschema.Schema {
	s, err := jsonschema.For[addReviewInput](nil)
	if err != nil {
		panic(fmt.Sprintf("add_review: infer input schema: %v", err))
	}

	s.Properties["rating"].Minimum = jsonschema.Ptr(float64(minReviewRating))
	s.Properties["rating"].Maximum = jsonschema.Ptr(float64(maxReviewRating))
	s.Properties["model"].MaxLength = jsonschema.Ptr(store.MaxModelLength)
	s.Properties["comment"].MaxLength = jsonschema.Ptr(store.MaxCommentLength)
	s.Properties["prompt"].MaxLength = jsonschema.Ptr(store.MaxPromptLength)
	s.Properties["image_url"].MaxLength = jsonschema.Ptr(store.MaxImageURLLength)
	s.Properties["image_url"].Format = "uri"
	s.Properties["agent_comment"].MaxLength = jsonschema.Ptr(store.MaxAgentCommentLength)

	return s
}

func getReviewsSchema() *jsonschema.Schema {
	s, err := jsonschema.For[getReviewsInput](nil)
	if err != nil {
		panic(fmt.Sprintf("get_reviews: infer input schema: %v", err))
	}

	s.Properties["model"].MaxLength = jsonschema.Ptr(store.MaxModelLength)

	return s
}

func getReviewSchema() *jsonschema.Schema {
	s, err := jsonschema.For[getReviewInput](nil)
	if err != nil {
		panic(fmt.Sprintf("get_review: infer input schema: %v", err))
	}

	return s
}

func deleteReviewSchema() *jsonschema.Schema {
	s, err := jsonschema.For[deleteReviewInput](nil)
	if err != nil {
		panic(fmt.Sprintf("delete_review: infer input schema: %v", err))
	}

	return s
}
