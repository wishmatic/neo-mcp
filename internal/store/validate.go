package store

import (
	"errors"
	"fmt"
	"net/url"
	"unicode"
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

func validateComment(comment string) error {
	if comment == "" {
		return errors.New("store: comment is required")
	}

	if utf8.RuneCountInString(comment) > MaxCommentLength {
		return fmt.Errorf("store: comment is longer than %d characters", MaxCommentLength)
	}

	for _, r := range comment {
		if unicode.IsControl(r) {
			return errors.New("store: comment must be a single line without control characters")
		}
	}

	return nil
}

func validateOptionalText(field, value string, max int) error {
	if value == "" {
		return nil
	}

	if utf8.RuneCountInString(value) > max {
		return fmt.Errorf("store: %s is longer than %d characters", field, max)
	}

	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return fmt.Errorf("store: %s must not contain control characters other than newlines or tabs", field)
		}
	}

	return nil
}

func validateImageURL(raw string) error {
	if raw == "" {
		return nil
	}

	if utf8.RuneCountInString(raw) > MaxImageURLLength {
		return fmt.Errorf("store: image url is longer than %d characters", MaxImageURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("store: image url %q must be an absolute http or https URL", raw)
	}

	return nil
}
