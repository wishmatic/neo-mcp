package novelai

import (
	"context"
	"time"
)

const (
	subscriptionPath = "/user/subscription"
	accountTimeout   = 30 * time.Second
)

type AnlasBalance struct {
	Total        int
	Subscription int
	Purchased    int
	UsagePercent *int
}

type subscriptionResponse struct {
	TrainingStepsLeft struct {
		Fixed     int `json:"fixedTrainingStepsLeft"`
		Purchased int `json:"purchasedTrainingSteps"`
	} `json:"trainingStepsLeft"`

	Usage struct {
		Percent *int `json:"percent"`
	} `json:"usage"`
}

// Anlas returns the account's Anlas balance. NovelAI moved this endpoint to the image host, so it shares baseURL.
func (c *Client) Anlas(ctx context.Context) (AnlasBalance, error) {
	ctx, cancel := context.WithTimeout(ctx, accountTimeout)
	defer cancel()

	var out subscriptionResponse
	if err := c.get(ctx, "anlas", subscriptionPath, &out); err != nil {
		return AnlasBalance{}, err
	}

	return AnlasBalance{
		Total:        out.TrainingStepsLeft.Fixed + out.TrainingStepsLeft.Purchased,
		Subscription: out.TrainingStepsLeft.Fixed,
		Purchased:    out.TrainingStepsLeft.Purchased,
		UsagePercent: out.Usage.Percent,
	}, nil
}
