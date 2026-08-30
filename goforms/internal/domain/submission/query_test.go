package submission_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

func TestListOptionsValidateAtTheDomainBoundary(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 30, 12, 0, 0, 123456000, time.UTC)
	end := start.Add(time.Hour)
	precise := start.Add(time.Nanosecond)
	invalidYear := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, options := range []submission.ListOptions{
		{Limit: 1}, {Limit: 100, Status: model.SubmissionStatusAccepted, SchemaVersion: 2},
		{Limit: 25, ReceivedFrom: &start, ReceivedBefore: &end},
		{Limit: 1, Before: start, BeforeID: "11111111-1111-4111-8111-111111111111"},
	} {
		require.NoError(t, options.Validate())
	}
	for _, options := range []submission.ListOptions{
		{}, {Limit: -1}, {Limit: 101}, {Limit: 1, Status: "processing"},
		{Limit: 1, SchemaVersion: -1}, {Limit: 1, SchemaVersion: submission.MaxSchemaVersion + 1},
		{Limit: 1, ReceivedFrom: &end, ReceivedBefore: &start}, {Limit: 1, ReceivedFrom: &start, ReceivedBefore: &start},
		{Limit: 1, ReceivedFrom: &precise}, {Limit: 1, ReceivedBefore: &invalidYear},
		{Limit: 1, Before: start}, {Limit: 1, BeforeID: "11111111-1111-4111-8111-111111111111"},
		{Limit: 1, Before: start, BeforeID: "invalid"},
	} {
		require.Error(t, options.Validate())
	}
}
