package present

import "fmt"

func Anlas(total, subscription, purchased int, usagePercent *int) string {
	summary := fmt.Sprintf(
		"Anlas: %d (subscription %d, purchased %d)",
		total, subscription, purchased,
	)

	if usagePercent != nil {
		summary += fmt.Sprintf("; V5 usage %d%%", *usagePercent)
	}

	return summary
}
