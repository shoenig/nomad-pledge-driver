package util

import (
	"os/user"
	"strconv"
	"sync"
)

// Users provide a way to interact with system users.
type Users interface {
	Current() (string, int, int, error)
}

func NewUsers() Users {
	return new(usersCache)
}

type usersCache struct {
	lock sync.Mutex

	username string
	uid      int
	gid      int
}

func (u *usersCache) Current() (string, int, int, error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.username != "" {
		return u.username, u.uid, u.gid, nil
	}

	c, err := user.Current()
	if err != nil {
		return "", 0, 0, err
	}

	uid, uErr := strconv.Atoi(c.Uid)
	if uErr != nil {
		return "", 0, 0, err
	}

	gid, gErr := strconv.Atoi(c.Gid)
	if gErr != nil {
		return "", 0, 0, err
	}

	u.username = c.Username
	u.uid = uid
	u.gid = gid

	return u.username, u.uid, u.gid, nil
}
