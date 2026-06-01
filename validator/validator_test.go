package validator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bosonicalcom/bedrock-go/validator"
	"github.com/bosonicalcom/bedrock-go/validator/validatortest"
)

func TestSetGlobalValidator_SwapsDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := validatortest.NewMockValidator(ctrl)

	sentinel := errors.New("from mock")
	mock.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(sentinel)

	validator.SetGlobalValidator(mock)
	t.Cleanup(func() {
		validator.SetGlobalValidator(validator.GoPlaygroundValidator{})
	})

	err := validator.Validate(context.Background(), struct{}{})
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}
