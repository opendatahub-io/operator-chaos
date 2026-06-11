package chaostransport

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// ActionFaultConfig defines fault injection for a specific action in the reconciler pipeline.
type ActionFaultConfig struct {
	// Skip makes the action a no-op (returns nil without running).
	Skip bool
	// FailBefore returns an error before the action runs.
	FailBefore string
	// FailAfter runs the action, then returns an error (simulates partial failure).
	// If the action itself also returns an error, both are joined.
	FailAfter string
	// Delay adds latency before the action runs (only when fault activates).
	Delay time.Duration
	// ErrorRate is the probability (0.0-1.0) that the fault activates.
	// A value of 0 means never fire. If not set (struct zero value),
	// defaults to 1.0 in NewActionInterceptor.
	ErrorRate float64
	// errorRateExplicit tracks whether ErrorRate was explicitly set.
	errorRateExplicit bool
}

// ActionInterceptor wraps action functions with chaos fault injection.
// It reads fault configuration for each action by matching action names.
type ActionInterceptor struct {
	faults []patternFault
}

type patternFault struct {
	pattern string
	config  ActionFaultConfig
}

// NewActionInterceptor creates an interceptor with the given fault configs.
// Keys are action name patterns (matched case-insensitively against the action's
// full qualified name, e.g., "deploy", "gc", "initialize").
// Patterns are sorted for deterministic matching when multiple patterns could match.
func NewActionInterceptor(faults map[string]ActionFaultConfig) *ActionInterceptor {
	sorted := make([]patternFault, 0, len(faults))
	for k, v := range faults {
		if !v.errorRateExplicit && v.ErrorRate == 0 {
			v.ErrorRate = 1.0
		}
		sorted = append(sorted, patternFault{
			pattern: strings.ToLower(k),
			config:  v,
		})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if len(sorted[i].pattern) != len(sorted[j].pattern) {
			return len(sorted[i].pattern) > len(sorted[j].pattern)
		}
		return sorted[i].pattern < sorted[j].pattern
	})
	return &ActionInterceptor{faults: sorted}
}

// WithErrorRate creates an ActionFaultConfig with an explicitly set error rate.
// Use this to set ErrorRate=0 (never fire) without it being overridden to 1.0.
func WithErrorRate(rate float64) func(*ActionFaultConfig) {
	return func(c *ActionFaultConfig) {
		c.ErrorRate = rate
		c.errorRateExplicit = true
	}
}

// ActionFn is a generic action function type matching the opendatahub-operator's actions.Fn.
type ActionFn func(ctx context.Context, rr interface{}) error

// Wrap wraps an action function with fault injection.
// The actionName is used to look up fault configuration.
func (ai *ActionInterceptor) Wrap(actionName string, fn ActionFn) ActionFn {
	return func(ctx context.Context, rr interface{}) error {
		fc := ai.matchFault(actionName)
		if fc == nil {
			return fn(ctx, rr)
		}

		if fc.ErrorRate < 1.0 && rand.Float64() > fc.ErrorRate {
			return fn(ctx, rr)
		}

		if fc.Skip {
			return nil
		}

		if fc.Delay > 0 {
			select {
			case <-time.After(fc.Delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if fc.FailBefore != "" {
			return &ChaosError{
				Operation: "action",
				Message:   fmt.Sprintf("chaos-action(%s): %s", actionName, fc.FailBefore),
			}
		}

		actionErr := fn(ctx, rr)

		if fc.FailAfter != "" {
			chaosErr := &ChaosError{
				Operation: "action",
				Message:   fmt.Sprintf("chaos-action(%s): %s", actionName, fc.FailAfter),
			}
			return errors.Join(actionErr, chaosErr)
		}

		return actionErr
	}
}

func (ai *ActionInterceptor) matchFault(actionName string) *ActionFaultConfig {
	lower := strings.ToLower(actionName)
	for i := range ai.faults {
		if strings.Contains(lower, ai.faults[i].pattern) {
			return &ai.faults[i].config
		}
	}
	return nil
}
