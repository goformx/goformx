package web

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

var submissionTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$`)

const maxSubmissionQueryBytes = 4096

func submissionListOptions(c echo.Context) (submission.ListOptions, error) {
	if len(c.Request().URL.RawQuery) > maxSubmissionQueryBytes {
		return submission.ListOptions{}, fmt.Errorf("submission query must not exceed %d bytes", maxSubmissionQueryBytes)
	}
	parameters, err := url.ParseQuery(c.Request().URL.RawQuery)
	if err != nil {
		return submission.ListOptions{}, errors.New("submission query encoding is invalid")
	}
	for name, values := range parameters {
		switch name {
		case "limit", "cursor", "receivedFrom", "receivedBefore", "status", "schemaVersion":
		default:
			return submission.ListOptions{}, errors.New("unsupported submission filter")
		}
		if len(values) != 1 {
			return submission.ListOptions{}, errors.New("submission filters must not be repeated")
		}
		if values[0] == "" && name != "cursor" && name != "limit" {
			return submission.ListOptions{}, errors.New("submission filters must not be empty")
		}
	}
	return submissionOptionsFromParameters(parameters)
}

func submissionOptionsFromParameters(parameters url.Values) (submission.ListOptions, error) {
	limit, err := submissionPageLimit(parameters.Get("limit"))
	if err != nil {
		return submission.ListOptions{}, err
	}
	before, beforeID, err := decodeSubmissionCursor(parameters.Get("cursor"))
	if err != nil {
		return submission.ListOptions{}, err
	}
	options := submission.ListOptions{Limit: limit, Before: before, BeforeID: beforeID,
		Status: model.SubmissionStatus(parameters.Get("status"))}
	if parameters.Get("schemaVersion") != "" {
		options.SchemaVersion, err = boundedInt(parameters.Get("schemaVersion"), 0, 1, submission.MaxSchemaVersion, "submission schema version")
		if err != nil {
			return submission.ListOptions{}, err
		}
	}
	for name, target := range map[string]**time.Time{
		"receivedFrom": &options.ReceivedFrom, "receivedBefore": &options.ReceivedBefore,
	} {
		if value := parameters.Get(name); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil || !submissionTimePattern.MatchString(value) {
				return submission.ListOptions{}, errors.New("submission time filters must be RFC 3339 timestamps with an offset")
			}
			*target = &parsed
		}
	}
	return options, options.Validate()
}
