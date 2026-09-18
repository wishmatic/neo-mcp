package store

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

func validateModel(model string) error {
	if model == "" {
		return errors.New("store: model is required")
	}

	if utf8.RuneCountInString(model) > MaxModelLength {
		return fmt.Errorf("store: model is longer than %d characters", MaxModelLength)
	}

	return nil
}
