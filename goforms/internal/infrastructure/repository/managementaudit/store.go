package managementaudit

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/managementaudit"
)

const insertEvent = `INSERT INTO management_audit
	(audit_id, organization_id, subject_id, credential_class, credential_id, request_id, correlation_id,
	 event, target_id, related_id, scopes, expires_at, occurred_at, form_id, enabled)
	VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, NULLIF($10, ''), $11::jsonb, $12, $13, NULLIF($14, '')::uuid, $15)`

// AppendGORM uses the caller's current transaction, never a separate connection.
func AppendGORM(ctx context.Context, tx *gorm.DB, event managementaudit.Event) error {
	return appendEvent(ctx, event, func(ctx context.Context, query string, args ...any) error {
		_, err := tx.Statement.ConnPool.ExecContext(ctx, query, args...)
		return err
	})
}

func AppendPGX(ctx context.Context, tx pgx.Tx, event managementaudit.Event) error {
	return appendEvent(ctx, event, func(ctx context.Context, query string, args ...any) error {
		_, err := tx.Exec(ctx, query, args...)
		return err
	})
}

func appendEvent(ctx context.Context, event managementaudit.Event, execute func(context.Context, string, ...any) error) error {
	if err := event.Validate(); err != nil {
		return err
	}
	scopes := append([]auth.Scope{}, event.Scopes...)
	sort.Slice(scopes, func(i, j int) bool { return scopes[i] < scopes[j] })
	encoded, err := json.Marshal(scopes)
	if err != nil {
		return managementaudit.ErrInvalid
	}
	err = execute(ctx, insertEvent, event.ID, event.Actor.OrganizationID, event.Actor.SubjectID,
		string(event.Actor.CredentialClass), event.Actor.CredentialID, event.Actor.RequestID, event.Actor.CorrelationID,
		string(event.Kind), event.TargetID, event.RelatedID, string(encoded), event.ExpiresAt, event.OccurredAt.UTC(), event.FormID, event.Enabled)
	if err != nil {
		// Driver diagnostics can contain values. Do not return them to callers.
		return managementaudit.ErrUnavailable
	}
	return nil
}
