// Package validator provides the Validator interface and a global validation instance backed by go-playground/validator.
package validator

//go:generate go tool mockgen -source=validator.go -destination=validatortest/validator_mock.go -package=validatortest

import (
	"context"
	"sync"
)

// A Validator is a component used to validate inputs.
type Validator interface {
	// Validate performs validation on the given input `v`.
	Validate(ctx context.Context, v any) error
}

var (
	_globalValidator    Validator = GoPlaygroundValidator{}
	_globalValidatorMux           = &sync.Mutex{}
)

// SetGlobalValidator sets the global validator instance.
//
// Uses [github.com/go-playground/validator/v10] if not set.
func SetGlobalValidator(v Validator) {
	_globalValidatorMux.Lock()
	defer _globalValidatorMux.Unlock()
	_globalValidator = v
}

// Validate validates the input `v` using the global validator instance.
//
// Uses [github.com/go-playground/validator/v10] as default validation engine.
func Validate(ctx context.Context, v any) error {
	return _globalValidator.Validate(ctx, v)
}
