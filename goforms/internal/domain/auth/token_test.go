package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestServiceTokenScopesOwnersExpiryAndRevocation(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	require.NotContains(t, string(token.Hash[:]), plaintext)
	require.NoError(t, token.Authorize(plaintext, "owner-a", auth.ScopeFormsRead, now))
	require.Error(t, token.Authorize(plaintext, "owner-b", auth.ScopeFormsRead, now))
	require.Error(t, token.Authorize(plaintext, "owner-a", auth.ScopeFormsWrite, now))
	require.Error(t, token.Authorize("gfst_wrong", "owner-a", auth.ScopeFormsRead, now))
	require.Error(t, token.Authorize(plaintext, "owner-a", auth.ScopeFormsRead, now.Add(2*time.Hour)))
	token.Revoke(now)
	require.Error(t, token.Authorize(plaintext, "owner-a", auth.ScopeFormsRead, now))
}
