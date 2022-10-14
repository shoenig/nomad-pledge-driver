package resources

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Get(t *testing.T) {
	specs, err := Get()
	must.NoError(t, err)
	must.Positive(t, specs.MHz)
	must.Positive(t, specs.Cores)
}
