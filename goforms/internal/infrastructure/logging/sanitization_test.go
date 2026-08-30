package logging

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSharedSanitizerIsSafeForConcurrentRequests(t *testing.T) {
	sanitizer := NewSanitizer()
	var callers sync.WaitGroup
	for worker := range 8 {
		callers.Go(func() {
			for request := range 100 {
				value := fmt.Sprintf("request-%d-%d", worker, request)
				if sanitized := sanitizer.SanitizeField("request_id", value); sanitized != value {
					t.Errorf("safe identifier changed")
				}
			}
		})
	}
	callers.Wait()
	require.Equal(t, "****", sanitizer.SanitizeField("password", "private-canary"))
}

func TestDerivedRuntimeLoggersAreConcurrentAndPreserveMasking(t *testing.T) {
	core, observed := observer.New(zapcore.InfoLevel)
	factory, err := NewFactory(&FactoryConfig{AppName: "concurrency-test", Environment: "production", LogLevel: "info"}, nil)
	require.NoError(t, err)
	root, err := factory.WithTestCore(core).CreateLogger()
	require.NoError(t, err)
	var callers sync.WaitGroup
	for worker := range 8 {
		callers.Go(func() {
			derived := root.WithComponent(fmt.Sprintf("worker-%d", worker))
			for request := range 100 {
				derived.WithRequestID(fmt.Sprintf("request-%d-%d", worker, request)).Info("concurrent event", "password", "private-canary")
			}
		})
	}
	callers.Wait()
	require.Equal(t, 800, observed.Len())
	for _, event := range observed.All() {
		require.Equal(t, "concurrent event", event.Message)
		require.Equal(t, "****", event.ContextMap()["password"])
		require.NotEmpty(t, event.ContextMap()["request_id"])
	}
}
