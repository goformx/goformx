package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/database"
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

func (s *Store) Save(ctx context.Context, token *auth.ServiceToken) error {
	if token == nil {
		return errors.New("service token is required")
	}
	scopes := make([]auth.Scope, 0, len(token.Scopes))
	for scope := range token.Scopes {
		scopes = append(scopes, scope)
	}
	row := record{TokenID: token.ID, Name: token.Name, OwnerID: token.OwnerID, TokenHash: token.Hash[:], Scopes: scopes,
		CreatedAt: token.CreatedAt, ExpiresAt: token.ExpiresAt, RevokedAt: token.RevokedAt}
	if err := s.db.GetDB().WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("save service token: %w", err)
	}
	return nil
}

func (s *Store) FindByID(ctx context.Context, tokenID string) (*auth.ServiceToken, error) {
	var row record
	if err := s.db.GetDB().WithContext(ctx).Where("token_id = ?", tokenID).First(&row).Error; err != nil {
		return nil, fmt.Errorf("find service token: %w", err)
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
		return fmt.Errorf("mark service token used: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Revoke(ctx context.Context, tokenID string, now time.Time) error {
	result := s.db.GetDB().WithContext(ctx).Model(&record{}).Where("token_id = ?", tokenID).
		Updates(map[string]any{"revoked_at": now.UTC(), "revocation_reason": "operator_revoked"})
	if result.Error != nil {
		return fmt.Errorf("revoke service token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByOrganization returns bounded service-token metadata without secret hashes.
func (s *Store) ListByOrganization(ctx context.Context, organizationID string, limit int) ([]*auth.ServiceToken, error) {
	if organizationID == "" || limit < 1 || limit > 100 {
		return nil, errors.New("organization and a limit between 1 and 100 are required")
	}
	var rows []record
	if err := s.db.GetDB().WithContext(ctx).
		Select("token_id", "name", "organization_id", "scopes", "created_at", "expires_at", "revoked_at", "last_used_at", "replaced_by_token_id", "revocation_reason").
		Where("organization_id = ?", organizationID).
		Order("created_at DESC, token_id DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list service tokens: %w", err)
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
	return tokens, nil
}

// RevokeByOrganization prevents a token ID from crossing an organization boundary.
func (s *Store) RevokeByOrganization(ctx context.Context, organizationID, tokenID string, now time.Time) error {
	result := s.db.GetDB().WithContext(ctx).Model(&record{}).Where(
		"organization_id = ? AND token_id = ?", organizationID, tokenID,
	).Updates(map[string]any{
		"revoked_at":        gorm.Expr("COALESCE(revoked_at, ?)", now.UTC()),
		"revocation_reason": gorm.Expr("COALESCE(revocation_reason, ?)", "api_revoked"),
	})
	if result.Error != nil {
		return fmt.Errorf("revoke organization service token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
