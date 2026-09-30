package repository_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/site"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestSiteAssociationAndRetryInPostgres(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GOFORMX_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	owner, foreign := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM forms WHERE organization_id IN (?, ?)", owner, foreign).Error
		_ = db.Exec("DELETE FROM sites WHERE organization_id IN (?, ?)", owner, foreign).Error
	})
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	store := formrepository.NewStore(&integrationDB{db: db}, logger)
	first, err := site.New(owner, "Personal site", "https://Example.COM:0443")
	require.NoError(t, err)
	created, newSite, err := store.CreateSite(t.Context(), first)
	require.NoError(t, err)
	require.True(t, newSite)
	retry, err := site.New(owner, "Personal site", "https://example.com")
	require.NoError(t, err)
	resolved, newSite, err := store.CreateSite(t.Context(), retry)
	require.NoError(t, err)
	require.False(t, newSite)
	require.Equal(t, created.ID, resolved.ID)
	conflict, err := site.New(owner, "Different name", "https://example.com")
	require.NoError(t, err)
	_, _, err = store.CreateSite(t.Context(), conflict)
	require.ErrorIs(t, err, common.ErrConflict)

	foreignSite, err := site.New(foreign, "Foreign", "https://foreign.example")
	require.NoError(t, err)
	foreignSite, _, err = store.CreateSite(t.Context(), foreignSite)
	require.NoError(t, err)
	_, err = store.GetSite(t.Context(), owner, foreignSite.ID)
	require.ErrorIs(t, err, common.ErrNotFound)
	listed, total, err := store.ListSites(t.Context(), owner, 25, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, created.ID, listed[0].ID)

	form := model.NewForm(owner, "Contact", "", model.JSON{"type": "object"})
	form.Name = "contact-" + uuid.NewString()[:8]
	form.SiteID = &foreignSite.ID
	logger.EXPECT().Error("failed to create form", "form_id", form.ID)
	require.ErrorIs(t, store.CreateForm(t.Context(), form), common.ErrNotFound)
	form.SiteID = &created.ID
	require.NoError(t, store.CreateForm(t.Context(), form))
	loaded, err := store.GetFormByID(t.Context(), owner, form.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, *loaded.SiteID)
	loaded.SiteID = &foreignSite.ID
	require.ErrorIs(t, store.UpdateForm(t.Context(), loaded, loaded.UpdatedAt), common.ErrNotFound)
	loaded.SiteID = &created.ID
	require.NoError(t, store.UpdateForm(t.Context(), loaded, loaded.UpdatedAt))

	// The FK protects non-HTTP writers as well as the repository path.
	require.Error(t, db.Exec("UPDATE forms SET site_id = ? WHERE uuid = ?", foreignSite.ID, form.ID).Error)
	var persisted model.Form
	require.NoError(t, db.Where("uuid = ?", form.ID).First(&persisted).Error)
	require.Equal(t, created.ID, *persisted.SiteID)
	loaded, err = store.GetFormByID(t.Context(), owner, form.ID)
	require.NoError(t, err)
	loaded.SiteID, loaded.SiteIDSet = nil, true
	require.NoError(t, store.UpdateForm(t.Context(), loaded, loaded.UpdatedAt))
	cleared, err := store.GetFormByID(t.Context(), owner, form.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.SiteID)
	// Preserve a live association for the populated rollback refusal probe.
	cleared.SiteID = &created.ID
	cleared.SiteIDSet = true
	require.NoError(t, store.UpdateForm(t.Context(), cleared, cleared.UpdatedAt))

	// A rollback with live identities must fail while preserving both the site
	// and the association. The down script locks forms and sites before checking.
	downSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "postgresql", "2026092902_sites.down.sql"))
	require.NoError(t, err)
	connection, err := pgx.Connect(t.Context(), databaseURL)
	require.NoError(t, err)
	_, err = connection.Exec(t.Context(), string(downSQL))
	require.ErrorContains(t, err, "sites rollback refused")
	_, rollbackErr := connection.Exec(t.Context(), "ROLLBACK")
	require.NoError(t, rollbackErr)
	require.NoError(t, connection.Close(t.Context()))
	_, err = store.GetSite(t.Context(), owner, created.ID)
	require.NoError(t, err)
	loaded, err = store.GetFormByID(t.Context(), owner, form.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, *loaded.SiteID)
}
