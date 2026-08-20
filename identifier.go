package benefit

import (
	"fmt"
	"regexp"
)

var namespacedValuePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*(\.[a-z0-9][a-z0-9_-]*)+$`)

func validateNamespacedValue(kind, value string) error {
	if !namespacedValuePattern.MatchString(value) {
		return fmt.Errorf("benefit: invalid %s %q", kind, value)
	}
	return nil
}
