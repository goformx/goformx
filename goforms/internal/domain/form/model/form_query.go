package model

import "errors"

var ErrPreconditionFailed = errors.New("form precondition failed")

type FormSort string

const (
	FormSortCreatedDesc FormSort = "-createdAt"
	FormSortCreatedAsc  FormSort = "createdAt"
	FormSortUpdatedDesc FormSort = "-updatedAt"
	FormSortUpdatedAsc  FormSort = "updatedAt"
	FormSortNameDesc    FormSort = "-name"
	FormSortNameAsc     FormSort = "name"
)

func (s FormSort) IsValid() bool {
	switch s {
	case FormSortCreatedDesc, FormSortCreatedAsc, FormSortUpdatedDesc, FormSortUpdatedAsc, FormSortNameDesc, FormSortNameAsc:
		return true
	default:
		return false
	}
}

type FormListOptions struct {
	Status LifecycleStatus
	Query  string
	Sort   FormSort
	Limit  int
	Offset int
}

func (o FormListOptions) Validate() error {
	if o.Status != "" && !o.Status.IsValid() {
		return errors.New("form status filter is invalid")
	}
	if len(o.Query) > 100 {
		return errors.New("form search query must not exceed 100 characters")
	}
	if !o.Sort.IsValid() {
		return errors.New("form sort is invalid")
	}
	if o.Limit < 1 || o.Limit > 100 {
		return errors.New("form page limit must be between 1 and 100")
	}
	if o.Offset < 0 || o.Offset > 10000 {
		return errors.New("form page offset must be between 0 and 10000")
	}
	return nil
}
