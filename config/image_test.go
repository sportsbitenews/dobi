package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ImageConfigSuite struct {
	suite.Suite
	image *ImageConfig
}

func TestImageConfigSuite(t *testing.T) {
	suite.Run(t, new(ImageConfigSuite))
}

func (s *ImageConfigSuite) SetupTest() {
	s.image = NewImageConfig()
	s.image.Dockerfile = "Dockerfile"
	s.image.Context = "."
	s.image.Image = "example"
}

func (s *ImageConfigSuite) TestString() {
	s.image.Context = "./files"
	s.Equal("Build image 'example' from 'files/Dockerfile'", s.image.String())
}

func (s *ImageConfigSuite) TestValidateMissingDependencies() {
	s.image.Depends = []string{"one", "two"}
	conf := NewConfig()
	err := ValidateResourcesExist(NewPath(""), conf, s.image.Dependencies())
	s.Error(err)
	s.Contains(err.Error(), "missing dependencies: one, two")
}

func (s *ImageConfigSuite) TestValidateMissingOneOfRequired() {
	s.image.Dockerfile = ""
	s.image.Context = ""

	conf := NewConfig()
	err := s.image.Validate(NewPath(""), conf)
	s.Error(err)
	s.Contains(err.Error(), "one of dockerfile, context, or pull is required")

}

func TestPullWithDuration(t *testing.T) {
	p := pull{}
	now := time.Now()
	old := now.Add(-time.Duration(32 * 60 * 10e9))
	p.TransformConfig(reflect.ValueOf("30m"))

	assert.Equal(t, p.Required(&now), false)
	assert.Equal(t, p.Required(&old), true)
	assert.Equal(t, p.Required(nil), true)
}

func TestPullTransformConfig(t *testing.T) {
	p := pull{}
	zero := reflect.Value{}
	err := p.TransformConfig(zero)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "must be a string")
}
