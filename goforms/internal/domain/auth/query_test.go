package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestTokenListOptionsRequireBoundedCompleteCursor(t *testing.T) {
	t.Parallel()
	require.NoError(t, (auth.TokenListOptions{Limit: auth.DefaultTokenPageLimit}).Validate())
	require.NoError(t, (auth.TokenListOptions{Limit: auth.MaxTokenPageLimit,
		Before: time.Now().UTC(), BeforeID: "abcdefghijklmnop"}).Validate())
	for _, options := range []auth.TokenListOptions{
		{}, {Limit: auth.MaxTokenPageLimit + 1}, {Limit: 1, Before: time.Now().UTC()}, {Limit: 1, BeforeID: "abcdefghijklmnop"},
	} {
		require.Error(t, options.Validate())
	}
}
