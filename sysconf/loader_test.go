package sysconf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/bosonicalcom/bedrock-go/sysconf"
	"github.com/bosonicalcom/bedrock-go/validator/validatortest"
)

type testConf struct {
	Host string `env:"TEST_HOST"`
	Port int    `env:"TEST_PORT"`
}

type LoaderSuite struct {
	suite.Suite
	ctrl *gomock.Controller
	mock *validatortest.MockValidator
}

func (s *LoaderSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mock = validatortest.NewMockValidator(s.ctrl)
}

func (s *LoaderSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *LoaderSuite) TestLoad_ParsesEnvVars() {
	s.T().Setenv("TEST_HOST", "localhost")
	s.T().Setenv("TEST_PORT", "8080")

	cfg, err := sysconf.Load[testConf](context.Background())
	s.Require().NoError(err)
	s.Equal("localhost", cfg.Host)
	s.Equal(8080, cfg.Port)
}

func (s *LoaderSuite) TestLoad_NoValidator_SkipsValidation() {
	s.T().Setenv("TEST_HOST", "")
	s.T().Setenv("TEST_PORT", "0")

	// mock has no EXPECT — gomock will fail the test if Validate is called unexpectedly
	_, err := sysconf.Load[testConf](context.Background())
	s.Require().NoError(err)
}

func (s *LoaderSuite) TestLoad_WithValidator_CallsValidate() {
	s.T().Setenv("TEST_HOST", "db.example.com")
	s.T().Setenv("TEST_PORT", "5432")

	s.mock.EXPECT().
		Validate(gomock.Any(), gomock.AssignableToTypeOf(testConf{})).
		Return(nil)

	cfg, err := sysconf.Load[testConf](context.Background(), sysconf.WithValidator(s.mock))
	s.Require().NoError(err)
	s.Equal("db.example.com", cfg.Host)
}

func (s *LoaderSuite) TestLoad_WithValidator_PropagatesError() {
	s.T().Setenv("TEST_HOST", "x")
	s.T().Setenv("TEST_PORT", "1")

	validationErr := errors.New("invalid config")
	s.mock.EXPECT().
		Validate(gomock.Any(), gomock.Any()).
		Return(validationErr)

	_, err := sysconf.Load[testConf](context.Background(), sysconf.WithValidator(s.mock))
	s.ErrorIs(err, validationErr)
}

func TestLoaderSuite(t *testing.T) {
	suite.Run(t, new(LoaderSuite))
}
