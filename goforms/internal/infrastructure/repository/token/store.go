package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/managementaudit"
	"github.com/goformx/goforms/internal/infrastructure/database"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
	auditstore "github.com/goformx/goforms/internal/infrastructure/repository/managementaudit"
)

type Store struct{ db database.DB }

func NewStore(db database.DB) *Store { return &Store{db: db} }

type record struct {
	TokenID           string       `gorm:"column:token_id;primaryKey"`
	Name              string       `gorm:"column:name"`
	OwnerID           string       `gorm:"column:organization_id"`
	TokenHash         []byte       `gorm:"column:token_hash"`
	Scopes            []auth.Scope `gorm:"serializer:json;type:jsonb"`
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	LastUsedAt        *time.Time
	ReplacedByTokenID *string `gorm:"column:replaced_by_token_id"`
	RevocationReason  *string `gorm:"column:revocation_reason"`
}

func (record) TableName() string { return "service_tokens" }

func (s *Store) Save(ctx context.Context, token *auth.ServiceToken, actor auth.AuditActor) error {
	if token == nil {
		return errors.New("service token is required")
	}
	if actor.Validate() != nil || actor.OrganizationID != token.OwnerID {
		return managementaudit.ErrInvalid
	}
	scopes := make([]auth.Scope, 0, len(token.Scopes))
	for scope := range token.Scopes {
		scopes = append(scopes, scope)
	}
	row := record{TokenID: token.ID, Name: token.Name, OwnerID: token.OwnerID, TokenHash: token.Hash[:], Scopes: scopes,
		CreatedAt: token.CreatedAt, ExpiresAt: token.ExpiresAt, RevokedAt: token.RevokedAt}
	event := managementaudit.Event{ID: uuid.NewString(), Actor: actor, Kind: managementaudit.TokenCreated,
		TargetID: token.ID, Scopes: scopes, ExpiresAt: &token.ExpiresAt, OccurredAt: token.CreatedAt}
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return auditstore.AppendGORM(ctx, tx, event)
	})
	if err != nil {
		return fmt.Errorf("save service token: %w", common.NewDatabaseError("save", "service token", token.ID, err))
	}
	return nil
}

func (s *Store) FindByID(ctx context.Context, tokenID string) (*auth.ServiceToken, error) {
	var row record
	if err := s.db.GetDB().WithContext(ctx).Where("token_id = ?", tokenID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find service token: %w", common.NewNotFoundErrorWithCause("find", "service token", tokenID, err))
		}
		return nil, fmt.Errorf("find service token: %w", common.NewDatabaseError("find", "service token", tokenID, err))
	}
	if len(row.TokenHash) != 32 {
		return nil, errors.New("stored service token hash is invalid")
	}
	var hash [32]byte
	copy(hash[:], row.TokenHash)
	scopes := make(map[auth.Scope]struct{}, len(row.Scopes))
	for _, scope := range row.Scopes {
		scopes[scope] = struct{}{}
	}
	loaded := &auth.ServiceToken{ID: row.TokenID, Name: row.Name, OwnerID: row.OwnerID, Hash: hash, Scopes: scopes,
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
		LastUsedAt: row.LastUsedAt}
	if row.ReplacedByTokenID != nil {
		loaded.ReplacedByTokenID = *row.ReplacedByTokenID
	}
	if row.RevocationReason != nil {
		loaded.RevocationReason = *row.RevocationReason
	}
	return loaded, nil
}

func (s *Store) MarkUsed(ctx context.Context, tokenID string, now time.Time) error {
	result := s.db.GetDB().WithContext(ctx).Model(&record{}).Where(
		"token_id = ? AND revoked_at IS NULL AND expires_at > ?", tokenID, now.UTC(),
	).Update("last_used_at", now.UTC())
	if result.Error != nil {
		return fmt.Errorf("mark service token used: %w", common.NewDatabaseError("mark used", "service token", tokenID, result.Error))
	}
	if result.RowsAffected == 0 {
		return common.NewNotFoundErrorWithCause("mark used", "service token", tokenID, gorm.ErrRecordNotFound)
	}
	return nil
}

// ListByOrganization returns bounded service-token metadata without secret hashes.
func (s *Store) ListByOrganization(ctx context.Context, organizationID string, options auth.TokenListOptions) ([]*auth.ServiceToken, bool, error) {
	if organizationID == "" || options.Validate() != nil {
		return nil, false, common.NewInvalidInputError("list", "service token", organizationID,
			errors.New("organization and valid token list options are required"))
	}
	var rows []record
	query := s.db.GetDB().WithContext(ctx).
		Select("token_id", "name", "organization_id", "scopes", "created_at", "expires_at", "revoked_at", "last_used_at", "replaced_by_token_id", "revocation_reason").
		Where("organization_id = ?", organizationID).
		Order("created_at DESC, token_id DESC")
	if !options.Before.IsZero() {
		query = query.Where("(created_at, token_id) < (?, ?)", options.Before, options.BeforeID)
	}
	if err := query.Limit(options.Limit + 1).Find(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("list service tokens: %w", common.NewDatabaseError("list", "service token", organizationID, err))
	}
	hasMore := len(rows) > options.Limit
	if hasMore {
		rows = rows[:options.Limit]
	}
	tokens := make([]*auth.ServiceToken, 0, len(rows))
	for _, row := range rows {
		scopes := make(map[auth.Scope]struct{}, len(row.Scopes))
		for _, scope := range row.Scopes {
			scopes[scope] = struct{}{}
		}
		token := &auth.ServiceToken{ID: row.TokenID, Name: row.Name, OwnerID: row.OwnerID, Scopes: scopes,
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, LastUsedAt: row.LastUsedAt}
		if row.ReplacedByTokenID != nil {
			token.ReplacedByTokenID = *row.ReplacedByTokenID
		}
		if row.RevocationReason != nil {
			token.RevocationReason = *row.RevocationReason
		}
		tokens = append(tokens, token)
	}
	return tokens, hasMore, nil
}

// RevokeByOrganization prevents a token ID from crossing an organization boundary.
func (s *Store) RevokeByOrganization(ctx context.Context, organizationID, tokenID string, now time.Time, actor auth.AuditActor) error {
	if actor.Validate() != nil || actor.OrganizationID != organizationID {
		return managementaudit.ErrInvalid
	}
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row record
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND token_id = ?", organizationID, tokenID).First(&row).Error; err != nil {
			return err
		}
		if row.RevokedAt != nil {
			return nil
		}
		if err := tx.Model(&row).Updates(map[string]any{
			"revoked_at": now.UTC(), "revocation_reason": "api_revoked",
		}).Error; err != nil {
			return err
		}
		return auditstore.AppendGORM(ctx, tx, managementaudit.Event{
			ID: uuid.NewString(), Actor: actor, Kind: managementaudit.TokenRevoked, TargetID: tokenID, OccurredAt: now,
		})
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.NewNotFoundErrorWithCause("revoke", "service token", tokenID, err)
	}
	return common.NewDatabaseError("revoke", "service token", tokenID, err)
}
