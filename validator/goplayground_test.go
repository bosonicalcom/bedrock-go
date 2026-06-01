package validator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bosonicalcom/bedrock-go/validator"
)

type validInput struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"min=0,max=150"`
	Tag   string `json:"tag"   validate:"oneof=a b c"`
	Short string `json:"short" validate:"min=2"`
}

type GoPlaygroundValidatorSuite struct {
	suite.Suite
	v validator.GoPlaygroundValidator
}

func (s *GoPlaygroundValidatorSuite) TestValidate_Valid() {
	input := validInput{Name: "Alice", Email: "alice@example.com", Age: 30, Tag: "a", Short: "hi"}
	s.NoError(s.v.Validate(context.Background(), input))
}

func (s *GoPlaygroundValidatorSuite) TestValidate_FieldViolations() {
	tests := []struct {
		name       string
		input      validInput
		wantReason string
	}{
		{
			name:       "missing required field",
			input:      validInput{Email: "alice@example.com", Age: 0, Tag: "a", Short: "hi"},
			wantReason: syserr.ReasonRequiredValue,
		},
		{
			name:       "invalid email format",
			input:      validInput{Name: "Alice", Email: "not-an-email", Age: 0, Tag: "a", Short: "hi"},
			wantReason: syserr.ReasonInvalidFormat,
		},
		{
			name:       "int exceeds max",
			input:      validInput{Name: "Alice", Email: "alice@example.com", Age: 200, Tag: "a", Short: "hi"},
			wantReason: syserr.ReasonValueTooLarge,
		},
		{
			name:       "value not in oneof",
			input:      validInput{Name: "Alice", Email: "alice@example.com", Age: 0, Tag: "z", Short: "hi"},
			wantReason: syserr.ReasonNotOneOfValue,
		},
		{
			name:       "string too short",
			input:      validInput{Name: "Alice", Email: "alice@example.com", Age: 0, Tag: "a", Short: "x"},
			wantReason: syserr.ReasonInvalidLength,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.v.Validate(context.Background(), tt.input)
			s.Require().Error(err)
			s.True(syserr.Is(err, syserr.CodeInvalidArgument))

			se, ok := err.(*syserr.Error)
			s.Require().True(ok)

			var br syserr.BadRequest
			found := false
			for _, d := range se.Details {
				if v, ok := d.(syserr.BadRequest); ok {
					br = v
					found = true
					break
				}
			}
			s.Require().True(found, "expected BadRequest detail")
			s.Require().NotEmpty(br.Violations)
			s.Equal(tt.wantReason, br.Violations[0].Reason)
		})
	}
}

func TestGoPlaygroundValidatorSuite(t *testing.T) {
	suite.Run(t, new(GoPlaygroundValidatorSuite))
}
