package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestParseScopesRejectsUnknownAndDeduplicates(t *testing.T) {
	scopes, err := parseScopes("forms:write, forms:publish,forms:write")
	require.NoError(t, err)
	require.Equal(t, []auth.Scope{auth.ScopeFormsWrite, auth.ScopeFormsPublish}, scopes)
	_, err = parseScopes("admin")
	require.ErrorContains(t, err, "unsupported scope")
}
