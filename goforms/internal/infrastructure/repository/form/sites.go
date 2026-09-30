package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/goformx/goforms/internal/domain/site"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

// CreateSite returns the existing site for an exact same-origin, same-name retry.
// An origin paired with different metadata is a conflict and never silently reused.
func (s *Store) CreateSite(ctx context.Context, candidate *site.Site) (*site.Site, bool, error) {
	var result *site.Site
	created := false
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "organization_id"}, {Name: "origin"}}, DoNothing: true}).Create(candidate)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 1 {
			result, created = candidate, true
			return nil
		}
		var existing site.Site
		if err := tx.Where("organization_id = ? AND origin = ?", candidate.OrganizationID, candidate.Origin).First(&existing).Error; err != nil {
			return err
		}
		if existing.Name != candidate.Name {
			return common.NewConflictError("create", "site", existing.ID, errors.New("origin already has different metadata"))
		}
		result = &existing
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("create site: %w", common.NewDatabaseError("create", "site", candidate.ID, err))
	}
	return result, created, nil
}

func (s *Store) GetSite(ctx context.Context, organizationID, id string) (*site.Site, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, common.NewInvalidInputError("get", "site", id, err)
	}
	var result site.Site
	err := s.db.GetDB().WithContext(ctx).Where("organization_id = ? AND uuid = ?", organizationID, id).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.NewNotFoundError("get", "site", id)
	}
	if err != nil {
		return nil, common.NewDatabaseError("get", "site", id, err)
	}
	return &result, nil
}

func (s *Store) ListSites(ctx context.Context, organizationID string, limit, offset int) ([]*site.Site, int64, error) {
	query := s.db.GetDB().WithContext(ctx).Model(&site.Site{}).Where("organization_id = ?", organizationID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, common.NewDatabaseError("count", "site", "", err)
	}
	var rows []*site.Site
	if err := query.Order("created_at DESC, uuid DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, common.NewDatabaseError("list", "site", "", err)
	}
	return rows, total, nil
}

func validateSiteAssociation(tx *gorm.DB, organizationID string, siteID *string) error {
	if siteID == nil {
		return nil
	}
	if _, err := uuid.Parse(*siteID); err != nil {
		return common.NewInvalidInputError("associate", "site", *siteID, err)
	}
	var count int64
	if err := tx.Model(&site.Site{}).Where("organization_id = ? AND uuid = ?", organizationID, *siteID).Count(&count).Error; err != nil {
		return common.NewDatabaseError("associate", "site", *siteID, err)
	}
	if count == 0 {
		return common.NewNotFoundError("associate", "site", *siteID)
	}
	return nil
}
