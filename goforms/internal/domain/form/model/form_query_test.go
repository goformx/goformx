package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestFormListOptionsAreBoundedAndVersioned(t *testing.T) {
	t.Parallel()
	require.NoError(t, (model.FormListOptions{
		Status: model.LifecyclePublished, Sort: model.FormSortCreatedDesc, Limit: 25,
	}).Validate())
	require.Error(t, (model.FormListOptions{Sort: "random", Limit: 25}).Validate())
	require.Error(t, (model.FormListOptions{Sort: model.FormSortNameAsc, Limit: 101}).Validate())
	require.Error(t, (model.FormListOptions{Sort: model.FormSortNameAsc, Limit: 25, Offset: 10001}).Validate())
	require.Error(t, (model.FormListOptions{Status: "publishing", Sort: model.FormSortNameAsc, Limit: 25}).Validate())
}
