package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSubmissionLimiterKeepsIndependentBoundedFormBuckets(t *testing.T) {
	t.Parallel()
	limiter := newSubmissionLimiter(V1Limits{PublicSubmissionRPS: 0.001, PublicSubmissionBurst: 1})
	require.True(t, limiter.allow("form-a"))
	require.False(t, limiter.allow("form-a"))
	require.True(t, limiter.allow("form-b"))

	limiter.mu.Lock()
	limiter.entries = make(map[string]*formLimiterEntry, maxTrackedSubmissionForms)
	now := time.Now()
	limiter.now = func() time.Time { return now }
	for index := 0; index < maxTrackedSubmissionForms; index++ {
		limiter.entries[string(rune(index+1))] = &formLimiterEntry{
			touched: now.Add(-submissionLimiterIdleTTL),
		}
	}
	limiter.mu.Unlock()
	require.True(t, limiter.allow("form-c"))
	require.LessOrEqual(t, len(limiter.entries), maxTrackedSubmissionForms)
}
