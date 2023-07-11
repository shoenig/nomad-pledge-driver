package util

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestUsers_Current(t *testing.T) {
	uc := NewUsers()

	username, uid, gid, err := uc.Current()
	must.NoError(t, err)
	must.NotEq(t, "", username)
	must.Positive(t, uid)
	must.Positive(t, gid)

	username2, uid2, gid2, err2 := uc.Current()
	must.NoError(t, err2)
	must.Eq(t, username, username2)
	must.Eq(t, uid, uid2)
	must.Eq(t, gid, gid2)
}
