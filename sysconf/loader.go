// Package sysconf provides environment-based configuration loading with optional validation.
package sysconf

import (
	"context"

	"github.com/bosonicalcom/bedrock-go/validator"
	"github.com/caarlos0/env/v11"
)

// Load loads the configuration of type `T` from the environment, validating it using the provided validator.
func Load[T any](ctx context.Context, opts ...LoadOpt) (T, error) {
	options := &loadOptions{}
	for _, opt := range opts {
		opt(options)
	}
	v, err := env.ParseAs[T]()
	if err != nil {
		var zeroVal T
		return zeroVal, err
	}
	if options.validator != nil {
		if err := options.validator.Validate(ctx, v); err != nil {
			var zeroVal T
			return zeroVal, err
		}
	}
	return v, nil
}

type loadOptions struct {
	validator validator.Validator
}

// LoadOpt is a functional option for [Load].
type LoadOpt func(*loadOptions)

// WithValidator sets the validator for the [Load] options.
func WithValidator(validator validator.Validator) LoadOpt {
	return func(opts *loadOptions) {
		opts.validator = validator
	}
}
