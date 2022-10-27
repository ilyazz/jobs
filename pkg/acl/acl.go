package acl

import (
	"fmt"
	"strings"
	"sync"
)

type UserID string
type ObjectID string
type AccessType int

const (
	Unknown    = AccessType(0)
	FullAccess = AccessType(1)
	ReadAccess = AccessType(2)
)

type AccessRequest struct {
	Subject UserID
	Object  ObjectID
	Action  AccessType
}

type AccessControl struct {
	userLock       sync.RWMutex
	superUsers     map[UserID]struct{}
	superReadUsers map[UserID]struct{}

	objLock sync.RWMutex
	owners  map[ObjectID]UserID
}

func New() *AccessControl {
	return &AccessControl{
		superUsers:     make(map[UserID]struct{}),
		superReadUsers: make(map[UserID]struct{}),
		owners:         make(map[ObjectID]UserID),
	}
}

func (c *AccessControl) AddSuperUsers(ids []string, access AccessType) error {
	c.userLock.Lock()
	defer c.userLock.Unlock()

	switch access {
	case FullAccess:
		for _, id := range ids {
			c.superUsers[UserID(id)] = struct{}{}
		}
	case ReadAccess:
		for _, id := range ids {
			c.superReadUsers[UserID(id)] = struct{}{}
		}
	default:
		return fmt.Errorf("unknown access type: %v", access)
	}

	return nil
}

func (c *AccessControl) SetOwner(o ObjectID, u UserID) error {

	u = UserID(strings.TrimSpace(string(u)))
	if u == "" {
		return fmt.Errorf("invalid user: %q", u)
	}

	o = ObjectID(strings.TrimSpace(string(o)))
	if o == "" {
		return fmt.Errorf("invalid object: %q", u)
	}

	c.objLock.Lock()
	defer c.objLock.Unlock()

	c.owners[o] = u

	return nil
}

func (c *AccessControl) Remove(o ObjectID) error {
	c.objLock.Lock()
	defer c.objLock.Unlock()

	_, ok := c.owners[o]
	if !ok {
		return fmt.Errorf("object not found")
	}

	delete(c.owners, o)

	return nil
}

func (c *AccessControl) Check(r AccessRequest) bool {

	c.objLock.Lock()
	defer c.objLock.Unlock()

	u, ok := c.owners[r.Object]
	if !ok {
		return false
	}

	if u == r.Subject {
		return true
	}

	c.userLock.RLock()
	defer c.userLock.RUnlock()

	_, ok = c.superUsers[r.Subject]
	if ok {
		return true
	}

	if r.Action == ReadAccess {
		_, ok := c.superReadUsers[r.Subject]
		if ok {
			return true
		}
	}

	return false
}
