package submission

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goformx/goforms/internal/domain/form/model"
)

const (
	SensitiveAnnotation        = "x-goformx-sensitive"
	MaxSensitivePaths          = 128
	MaxSensitivePathCharacters = 256
)

var ErrRedactionPolicy = errors.New("submission redaction policy cannot be applied")

// SensitivePaths validates the explicit root policy. It does not guess at
// sensitive field names or reinterpret renderer/JSON Schema annotations.
func SensitivePaths(schema model.JSON) ([]string, error) {
	if schema == nil {
		return nil, ErrRedactionPolicy
	}
	raw, exists := schema[SensitiveAnnotation]
	if !exists {
		return []string{}, nil
	}
	var paths []string
	switch values := raw.(type) {
	case []string:
		paths = append([]string{}, values...)
	case []any:
		paths = make([]string, 0, len(values))
		for _, value := range values {
			path, ok := value.(string)
			if !ok {
				return nil, ErrRedactionPolicy
			}
			paths = append(paths, path)
		}
	default:
		return nil, ErrRedactionPolicy
	}
	if len(paths) > MaxSensitivePaths {
		return nil, ErrRedactionPolicy
	}
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if seen[path] || !utf8.ValidString(path) || utf8.RuneCountInString(path) > MaxSensitivePathCharacters {
			return nil, ErrRedactionPolicy
		}
		if _, err := pointerTokens(path); err != nil {
			return nil, err
		}
		seen[path] = true
	}
	sort.Strings(paths)
	return paths, nil
}

func pointerTokens(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, ErrRedactionPolicy
	}
	tokens := strings.Split(path[1:], "/")
	for index, token := range tokens {
		for position := 0; position < len(token); position++ {
			if token[position] == '~' {
				if position+1 == len(token) || (token[position+1] != '0' && token[position+1] != '1') {
					return nil, ErrRedactionPolicy
				}
				position++
			}
		}
		// RFC 6901 order matters: ~01 names the literal key ~1, not /.
		tokens[index] = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
	}
	return tokens, nil
}

// Redact returns a detached display/export projection. It never mutates the
// accepted payload or schema, and preserves exact numeric values and array order.
func Redact(schema, data model.JSON) (model.JSON, []string, error) {
	if data == nil {
		return nil, nil, ErrRedactionPolicy
	}
	paths, err := SensitivePaths(schema)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, nil, ErrRedactionPolicy
	}
	var result model.JSON
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, nil, ErrRedactionPolicy
	}
	// Resolve every policy against the original shape before mutating the copy.
	// Otherwise a parent removal could hide an invalid child selector.
	for _, path := range paths {
		tokens, _ := pointerTokens(path)
		if err := validateTraversal(map[string]any(result), tokens); err != nil {
			return nil, nil, err
		}
	}
	for _, path := range paths {
		tokens, _ := pointerTokens(path)
		if len(tokens) == 0 {
			return model.JSON{}, paths, nil
		}
		removePointer(map[string]any(result), tokens)
	}
	return result, paths, nil
}

func arrayIndex(token string) (int, error) {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, ErrRedactionPolicy
	}
	for _, character := range token {
		if character < '0' || character > '9' {
			return 0, ErrRedactionPolicy
		}
	}
	index, err := strconv.ParseUint(token, 10, 31)
	if err != nil {
		return 0, ErrRedactionPolicy
	}
	return int(index), nil
}

func validateTraversal(node any, tokens []string) error {
	if len(tokens) == 0 || node == nil {
		return nil
	}
	switch value := node.(type) {
	case map[string]any:
		child, exists := value[tokens[0]]
		if !exists {
			return nil
		}
		return validateTraversal(child, tokens[1:])
	case []any:
		index, err := arrayIndex(tokens[0])
		if err != nil {
			return err
		}
		if index >= len(value) {
			return nil
		}
		return validateTraversal(value[index], tokens[1:])
	default:
		return ErrRedactionPolicy
	}
}

func removePointer(node any, tokens []string) {
	if len(tokens) == 0 || node == nil {
		return
	}
	switch value := node.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			delete(value, tokens[0])
			return
		}
		removePointer(value[tokens[0]], tokens[1:])
	case []any:
		index, err := arrayIndex(tokens[0])
		if err != nil || index >= len(value) {
			return
		} // Traversal was already validated.
		if len(tokens) == 1 {
			value[index] = nil
			return
		}
		removePointer(value[index], tokens[1:])
	}
}
