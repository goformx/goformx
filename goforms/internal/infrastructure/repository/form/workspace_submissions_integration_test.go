package repository_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/site"
	"github.com/goformx/goforms/internal/domain/submission"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestWorkspaceSubmissionsPageInPostgres(t *testing.T) {
	dsn := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("GOFORMX_TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	store := formrepository.NewStore(&integrationDB{db: db}, mocklogging.NewMockLogger(gomock.NewController(t)))
	owner, foreign := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM form_submissions WHERE form_id IN (SELECT uuid FROM forms WHERE organization_id IN (?, ?))", owner, foreign).Error
		_ = db.Exec("DELETE FROM forms WHERE organization_id IN (?, ?)", owner, foreign).Error
		_ = db.Exec("DELETE FROM sites WHERE organization_id IN (?, ?)", owner, foreign).Error
	})
	siteA, err := site.New(owner, "Site A", "https://a.example.test")
	require.NoError(t, err)
	siteA, _, err = store.CreateSite(t.Context(), siteA)
	require.NoError(t, err)
	siteB, err := site.New(owner, "Site B", "https://b.example.test")
	require.NoError(t, err)
	siteB, _, err = store.CreateSite(t.Context(), siteB)
	require.NoError(t, err)
	foreignSite, err := site.New(foreign, "Other site", "https://other.example.test")
	require.NoError(t, err)
	foreignSite, _, err = store.CreateSite(t.Context(), foreignSite)
	require.NoError(t, err)
	createForm := func(org, name string, siteID *string) *model.Form {
		form := model.NewForm(org, name, name, model.JSON{"type": "object"})
		form.Name = name + "-" + uuid.NewString()[:8]
		form.SiteID = siteID
		require.NoError(t, store.CreateForm(t.Context(), form))
		return form
	}
	a := createForm(owner, "Site A form", &siteA.ID)
	b := createForm(owner, "Site B form", &siteB.ID)
	legacy := createForm(owner, "Legacy form", nil)
	f := createForm(foreign, "Foreign form", &foreignSite.ID)
	when := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	createRow := func(form *model.Form, at time.Time, id string) {
		require.NoError(t, db.Create(&model.FormSubmission{ID: id, FormID: form.ID, SchemaVersion: 1, Data: model.JSON{"message": "test"}, SubmittedAt: at}).Error)
	}
	firstID, secondID := "ffffffff-ffff-4fff-8fff-ffffffffffff", "11111111-1111-4111-8111-111111111111"
	createRow(a, when, secondID)
	createRow(b, when, firstID)
	createRow(legacy, when.Add(-time.Second), uuid.NewString())
	createRow(f, when.Add(time.Second), uuid.NewString())
	page, more, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, submission.WorkspaceListOptions{ListOptions: submission.ListOptions{Limit: 1}})
	require.NoError(t, err)
	require.True(t, more)
	require.Len(t, page, 1)
	require.Equal(t, firstID, page[0].Submission.ID)
	require.Equal(t, b.Name, page[0].FormName)
	require.Equal(t, b.Title, page[0].FormTitle)
	require.Equal(t, siteB.ID, *page[0].SiteID)
	second, more, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, submission.WorkspaceListOptions{ListOptions: submission.ListOptions{Limit: 1, Before: page[0].Submission.SubmittedAt, BeforeID: page[0].Submission.ID}})
	require.NoError(t, err)
	require.True(t, more)
	require.Equal(t, secondID, second[0].Submission.ID)
	sitePage, more, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, submission.WorkspaceListOptions{ListOptions: submission.ListOptions{Limit: 25}, SiteID: siteA.ID})
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, sitePage, 1)
	require.Equal(t, a.ID, sitePage[0].Submission.FormID)
	formPage, more, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, submission.WorkspaceListOptions{
		ListOptions: submission.ListOptions{Limit: 25, ReceivedFrom: &when, Status: model.SubmissionStatusAccepted, SchemaVersion: 1},
		SiteID:      siteB.ID, FormID: b.ID,
	})
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, formPage, 1)
	require.Equal(t, firstID, formPage[0].Submission.ID)
	all, _, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, submission.WorkspaceListOptions{ListOptions: submission.ListOptions{Limit: 25}})
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Nil(t, all[2].SiteID)
	for _, options := range []submission.WorkspaceListOptions{
		{ListOptions: submission.ListOptions{Limit: 25}, SiteID: foreignSite.ID},
		{ListOptions: submission.ListOptions{Limit: 25}, SiteID: uuid.NewString()},
		{ListOptions: submission.ListOptions{Limit: 25}, FormID: f.ID},
		{ListOptions: submission.ListOptions{Limit: 25}, FormID: uuid.NewString()},
		{ListOptions: submission.ListOptions{Limit: 25}, SiteID: siteA.ID, FormID: b.ID},
	} {
		_, _, err := store.ListWorkspaceSubmissionsPage(t.Context(), owner, options)
		require.ErrorIs(t, err, common.ErrNotFound)
	}
}
