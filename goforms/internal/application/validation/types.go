package validation

// Error is a stable, path-addressable JSON Schema validation error.
type Error struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is the implementation-independent validation result.
type Result struct {
	IsValid bool    `json:"is_valid"`
	Errors  []Error `json:"errors,omitempty"`
}
