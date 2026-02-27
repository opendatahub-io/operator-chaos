package injection

import (
	"fmt"
	"regexp"
)

const maxNameLength = 253

var validNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.\-]*[a-z0-9])?$`)
var validFieldPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.\-]*$`)

func validateK8sName(paramName, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("%s must not be empty", paramName)
	}
	if len(value) > maxNameLength {
		return fmt.Errorf("%s exceeds maximum length of %d characters", paramName, maxNameLength)
	}
	if !validNamePattern.MatchString(value) {
		return fmt.Errorf("%s %q is not a valid Kubernetes name (must match RFC 1123 DNS subdomain)", paramName, value)
	}
	return nil
}

func validateFieldName(paramName, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("%s must not be empty", paramName)
	}
	if len(value) > maxNameLength {
		return fmt.Errorf("%s exceeds maximum length of %d characters", paramName, maxNameLength)
	}
	if !validFieldPattern.MatchString(value) {
		return fmt.Errorf("%s %q is not a valid field name", paramName, value)
	}
	return nil
}
