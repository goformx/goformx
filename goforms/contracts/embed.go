// Package contracts exposes the exact published contract to runtime validation.
package contracts

import _ "embed"

//go:embed schema/form-definition.schema.json
var formDefinition string

// FormDefinition returns the immutable, embedded source contract without runtime file or network access.
func FormDefinition() string { return formDefinition }
