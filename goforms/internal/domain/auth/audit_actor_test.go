package auth

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuditActorRequiresExplicitBoundedIdentity(t *testing.T) {
	t.Parallel()
	valid := AuditActor{OrganizationID: uuid.NewString(), SubjectID: "-token-id", CredentialID: "-token-id",
		CredentialClass: CredentialClassServiceToken, RequestID: "_trace-id"}
	require.NoError(t, valid.Validate())
	for _, change := range []func(*AuditActor){
		func(a *AuditActor) { a.OrganizationID = "not-an-org" },
		func(a *AuditActor) { a.SubjectID = "" },
		func(a *AuditActor) { a.CredentialID = "different-token" },
		func(a *AuditActor) { a.CredentialClass = "implicit" },
		func(a *AuditActor) { a.RequestID = "canary\nsecret" },
		func(a *AuditActor) { a.RequestID = strings.Repeat("a", 129) },
	} {
		actor := valid
		change(&actor)
		err := actor.Validate()
		require.Error(t, err)
		require.NotContains(t, err.Error(), "canary")
	}
	require.Error(t, (AuditActor{}).Validate())
	require.Error(t, DatabaseAuditActor("", valid.OrganizationID).Validate())
	operator := DatabaseAuditActor("quoted role-名", valid.OrganizationID)
	require.NoError(t, operator.Validate())
	require.Equal(t, CredentialClassDatabaseOperator, operator.CredentialClass)
	require.Equal(t, operator.SubjectID, operator.CredentialID)
	require.NotEqual(t, "quoted role-名", operator.SubjectID)
}
