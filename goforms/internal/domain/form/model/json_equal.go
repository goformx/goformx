package model

import (
	"encoding/json"
	"math/big"
)

// EqualJSON compares JSON values, not their numeric spelling. PostgreSQL JSONB
// expands exponents, so an idempotent retry must treat 1e3 and 1000 as equal
// without treating adjacent large integers or decimals as the same value.
func EqualJSON(left, right JSON) bool {
	decode := func(value JSON) (any, error) {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var result any
		err = decodeExactJSON(encoded, &result)
		return result, err
	}
	l, err := decode(left)
	if err != nil {
		return false
	}
	r, err := decode(right)
	return err == nil && equalJSONValues(l, r)
}

func equalJSONValues(left, right any) bool {
	switch value := left.(type) {
	case map[string]any:
		other, ok := right.(map[string]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for key, item := range value {
			candidate, exists := other[key]
			if !exists || !equalJSONValues(item, candidate) {
				return false
			}
		}
		return true
	case []any:
		other, ok := right.([]any)
		if !ok || len(value) != len(other) {
			return false
		}
		for index, item := range value {
			if !equalJSONValues(item, other[index]) {
				return false
			}
		}
		return true
	case json.Number:
		other, ok := right.(json.Number)
		if !ok {
			return false
		}
		l, leftOK := new(big.Rat).SetString(string(value))
		r, rightOK := new(big.Rat).SetString(string(other))
		return leftOK && rightOK && l.Cmp(r) == 0
	default:
		return left == right
	}
}
