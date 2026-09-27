// Package apperr defines HTTP-agnostic error sentinels shared by all domains.
// Services wrap them with context: fmt.Errorf("meter %s: %w", id, apperr.ErrNotFound).
package apperr

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrValidation = errors.New("validation failed")
)
