package assertionreplay

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/database"
)

type Store struct{ db database.DB }

func NewStore(db database.DB) *Store { return &Store{db: db} }

type record struct {
	Issuer         string    `gorm:"column:issuer;primaryKey"`
	AssertionID    string    `gorm:"column:assertion_id;primaryKey"`
	ExpiresAt      time.Time `gorm:"column:expires_at"`
	FirstSeenAt    time.Time `gorm:"column:first_seen_at"`
	SubjectID      string    `gorm:"column:subject_id"`
	OrganizationID string    `gorm:"column:organization_id"`
	KeyID          string    `gorm:"column:key_id"`
}

func (record) TableName() string { return "first_party_assertion_replays" }

// Consume atomically records an assertion identity before handler dispatch.
func (s *Store) Consume(ctx context.Context, replay auth.AssertionReplay) error {
	row := record{Issuer: replay.Issuer, AssertionID: replay.AssertionID,
		ExpiresAt: replay.ExpiresAt.UTC(), FirstSeenAt: replay.FirstSeenAt.UTC(),
		SubjectID: replay.SubjectID, OrganizationID: replay.OrganizationID, KeyID: replay.KeyID}
	if err := s.db.GetDB().WithContext(ctx).Create(&row).Error; err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return auth.ErrFirstPartyAssertionReplay
		}
		return fmt.Errorf("consume first-party assertion: %w", err)
	}
	return nil
}

// DeleteExpired removes only replay identities whose assertion and skew window have elapsed.
func (s *Store) DeleteExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	if limit < 1 || limit > 10000 {
		return 0, errors.New("replay cleanup limit must be between 1 and 10000")
	}
	result := s.db.GetDB().WithContext(ctx).Exec(`
		DELETE FROM first_party_assertion_replays
		WHERE (issuer, assertion_id) IN (
			SELECT issuer, assertion_id
			FROM first_party_assertion_replays
			WHERE expires_at < ?
			ORDER BY expires_at
			LIMIT ?
		)
	`, now.UTC(), limit)
	if result.Error != nil {
		return 0, fmt.Errorf("delete expired assertion replays: %w", result.Error)
	}
	return result.RowsAffected, nil
}
