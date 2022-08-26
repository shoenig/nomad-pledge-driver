package resources

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_Get(t *testing.T) {
	specs, err := Get()
	must.NoError(t, err)
	must.Greater(t, specs.MHz, 0)
	must.Greater(t, specs.Cores, 0)
}
