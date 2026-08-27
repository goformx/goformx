package security

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaFirstRoutesAreNotCookieCSRFProtected(t *testing.T) {
	require.True(t, IsAPIRoute("/v1/forms"))
	require.True(t, IsAPIRoute("/v1/forms/11111111-1111-4111-8111-111111111111/versions"))
	require.True(t, IsFormSubmissionRoute("/v1/public/forms/gfpk_example/submissions"))
	require.False(t, IsFormSubmissionRoute("/v1/public/forms/gfpk_example/schema"))
}
