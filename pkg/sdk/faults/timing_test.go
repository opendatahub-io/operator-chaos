package faults

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDelayConfig(t *testing.T) {
	spec := DelayConfig(2 * time.Second)

	assert.Equal(t, 2*time.Second, spec.Delay)
	assert.Equal(t, 0.0, spec.ErrorRate)
	assert.Equal(t, "", spec.Error)
}

func TestDelayConfigSmallDuration(t *testing.T) {
	spec := DelayConfig(100 * time.Millisecond)

	assert.Equal(t, 100*time.Millisecond, spec.Delay)
	assert.Equal(t, 0.0, spec.ErrorRate)
	assert.Equal(t, "", spec.Error)
}

func TestRandomDelayConfig(t *testing.T) {
	spec := RandomDelayConfig(500 * time.Millisecond)

	assert.Equal(t, 500*time.Millisecond, spec.MaxDelay)
	assert.Equal(t, time.Duration(0), spec.Delay)
	assert.Equal(t, 0.0, spec.ErrorRate)
	assert.Equal(t, "", spec.Error)
}

func TestRandomDelayConfigLargeDuration(t *testing.T) {
	spec := RandomDelayConfig(5 * time.Second)

	assert.Equal(t, 5*time.Second, spec.MaxDelay)
	assert.Equal(t, time.Duration(0), spec.Delay)
	assert.Equal(t, 0.0, spec.ErrorRate)
}

func TestRandomDelayConfig_UsesMaxDelay(t *testing.T) {
	spec := RandomDelayConfig(1 * time.Second)

	// MaxDelay should be set, Delay should be zero
	assert.Equal(t, 1*time.Second, spec.MaxDelay)
	assert.Equal(t, time.Duration(0), spec.Delay)

	// When used with FaultConfig.MaybeInject, the delay will be random in [0, MaxDelay)
	// We verify here that the spec is configured correctly for random behavior
	assert.Zero(t, spec.ErrorRate, "RandomDelayConfig should not set error rate")
	assert.Empty(t, spec.Error, "RandomDelayConfig should not set error message")
}

func TestDeadlineExceedConfig(t *testing.T) {
	spec := DeadlineExceedConfig(0.7)

	assert.Equal(t, time.Duration(0), spec.Delay)
	assert.Equal(t, 0.7, spec.ErrorRate)
	assert.Equal(t, "context deadline exceeded", spec.Error)
}

func TestDeadlineExceedConfigFullRate(t *testing.T) {
	spec := DeadlineExceedConfig(1.0)

	assert.Equal(t, 1.0, spec.ErrorRate)
	assert.Equal(t, "context deadline exceeded", spec.Error)
}
