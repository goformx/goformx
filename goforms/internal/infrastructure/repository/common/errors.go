// Package common provides shared utilities and types for repository implementations
// including error handling, pagination, and common data structures.
package common

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Common store errors
var (
	ErrNotFound      = errors.New("record not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrConflict      = errors.New("conflict")
	ErrDatabaseError = errors.New("database error")
)

// StoreError represents a store operation error
type StoreError struct {
	Op      string // Operation that failed
	Entity  string // Entity type (e.g., "user", "form")
	ID      string // Entity ID
	Kind    error  // Stable category exposed through errors.Is
	Err     error  // The underlying error
	Details string // Additional error details
}

// Error implements the error interface
func (e *StoreError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s %s", e.Op, e.Entity, e.ID)
	}

	// Always include the full error details
	errDetails := fmt.Sprintf("%+v", e.Err)
	if e.Details != "" {
		errDetails = fmt.Sprintf("%s\nDetails: %s", errDetails, e.Details)
	}

	return fmt.Sprintf("%s: %s %s\nError: %s", e.Op, e.Entity, e.ID, errDetails)
}

// Unwrap returns the underlying error
func (e *StoreError) Unwrap() []error {
	causes := make([]error, 0, 2)
	if e.Kind != nil {
		causes = append(causes, e.Kind)
	}
	if e.Err != nil && !errors.Is(e.Err, e.Kind) {
		causes = append(causes, e.Err)
	}
	return causes
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(op, entity, id string) error {
	return &StoreError{
		Op:     op,
		Entity: entity,
		ID:     id,
		Kind:   ErrNotFound,
		Err:    ErrNotFound,
	}
}

// NewNotFoundErrorWithCause retains a driver sentinel for internal callers while
// exposing only the stable not-found category to upper layers.
func NewNotFoundErrorWithCause(op, entity, id string, err error) error {
	return &StoreError{Op: op, Entity: entity, ID: id, Kind: ErrNotFound, Err: err}
}

// NewInvalidInputError creates a new invalid input error
func NewInvalidInputError(op, entity, id string, err error) error {
	return &StoreError{
		Op:      op,
		Entity:  entity,
		ID:      id,
		Kind:    ErrInvalidInput,
		Err:     err,
		Details: fmt.Sprintf("%+v", err),
	}
}

// NewConflictError creates a stable conflict error without requiring callers
// outside persistence to inspect database diagnostics.
func NewConflictError(op, entity, id string, err error) error {
	return &StoreError{Op: op, Entity: entity, ID: id, Kind: ErrConflict, Err: err}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(op, entity, id string, err error) error {
	// Create a detailed error message that includes all error information
	details := fmt.Sprintf("type: %T\nmessage: %s\ndetails: %+v",
		err,
		err.Error(),
		err,
	)

	return &StoreError{
		Op:      op,
		Entity:  entity,
		ID:      id,
		Kind:    databaseErrorKind(err),
		Err:     err,
		Details: details,
	}
}

func databaseErrorKind(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrConflict
	case errors.Is(err, gorm.ErrInvalidData):
		return ErrInvalidInput
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "23505":
			return ErrConflict
		case "22P02":
			return ErrInvalidInput
		}
	}
	return ErrDatabaseError
}
