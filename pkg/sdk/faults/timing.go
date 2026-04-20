package faults

import (
	"time"

	"github.com/opendatahub-io/operator-chaos/pkg/sdk"
)

// DelayConfig creates a fault that adds a fixed delay.
func DelayConfig(delay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		Delay: delay,
	}
}

// RandomDelayConfig creates a fault that adds a random delay in [0, maxDelay)
// at injection time. Each call to MaybeInject will sleep for a different
// randomly chosen duration. Use DelayConfig if you want a fixed delay instead.
func RandomDelayConfig(maxDelay time.Duration) sdk.FaultSpec {
	return sdk.FaultSpec{
		MaxDelay: maxDelay,
	}
}

// DeadlineExceedConfig creates a fault that simulates context deadline exceeded.
func DeadlineExceedConfig(rate float64) sdk.FaultSpec {
	return sdk.FaultSpec{
		ErrorRate: rate,
		Error:     "context deadline exceeded",
	}
}
