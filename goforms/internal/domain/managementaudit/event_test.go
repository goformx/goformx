package managementaudit

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

func TestEventRejectsMissingActorAndNonCanonicalMetadata(t *testing.T) {
	t.Parallel()
	expires := time.Now().Add(time.Hour)
	valid := Event{ID: uuid.NewString(), Actor: auth.DatabaseAuditActor("test-role", uuid.NewString()),
		Kind: TokenCreated, TargetID: "_token-id", Scopes: []auth.Scope{auth.ScopeFormsRead}, ExpiresAt: &expires, OccurredAt: time.Now()}
	require.NoError(t, valid.Validate())
	for _, change := range []func(*Event){
		func(e *Event) { e.ID = "" },
		func(e *Event) { e.Actor = auth.AuditActor{} },
		func(e *Event) { e.TargetID = "" },
		func(e *Event) { e.Kind = "request.body" },
		func(e *Event) { e.OccurredAt = time.Time{} },
		func(e *Event) { e.RelatedID = "unexpected" },
		func(e *Event) { e.Scopes = nil },
		func(e *Event) { e.Scopes = []auth.Scope{"canary-secret"} },
		func(e *Event) { e.Scopes = []auth.Scope{auth.ScopeFormsRead, auth.ScopeFormsRead} },
		func(e *Event) { e.ExpiresAt = nil },
		func(e *Event) { e.Kind = TokenRevoked },
		func(e *Event) { e.Kind = TokenRotated },
	} {
		event := valid
		change(&event)
		require.ErrorIs(t, event.Validate(), ErrInvalid)
	}
	valid.Kind, valid.RelatedID = TokenRotated, "replacement"
	require.NoError(t, valid.Validate())
	valid.Kind, valid.RelatedID, valid.ExpiresAt, valid.Scopes = TokenRevoked, "", nil, nil
	require.NoError(t, valid.Validate())
}
