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
	TokenID   string       `gorm:"column:token_id;primaryKey"`
	OwnerID   string       `gorm:"column:owner_id"`
	TokenHash []byte       `gorm:"column:token_hash"`
	Scopes    []auth.Scope `gorm:"serializer:json;type:jsonb"`
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
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
	row := record{TokenID: token.ID, OwnerID: token.OwnerID, TokenHash: token.Hash[:], Scopes: scopes,
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
	return &auth.ServiceToken{ID: row.TokenID, OwnerID: row.OwnerID, Hash: hash, Scopes: scopes,
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt}, nil
}

func (s *Store) Revoke(ctx context.Context, tokenID string, now time.Time) error {
	result := s.db.GetDB().WithContext(ctx).Model(&record{}).Where("token_id = ?", tokenID).
		Update("revoked_at", now.UTC())
	if result.Error != nil {
		return fmt.Errorf("revoke service token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
