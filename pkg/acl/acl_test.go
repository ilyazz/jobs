package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOk(t *testing.T) {
	acl := New()

	acl.SetOwner("obj1", "user1")
	acl.SetOwner("obj2", "user2")

	assert.True(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "obj1",
		Action:  FullAccess,
	}))

	assert.True(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "obj1",
		Action:  ReadAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "obj2",
		Action:  FullAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "obj2",
		Action:  ReadAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "no-such-obj",
		Action:  FullAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "user1",
		Object:  "no-such-obj",
		Action:  ReadAccess,
	}))
}

func TestSuperRead(t *testing.T) {
	acl := New()

	acl.SetOwner("obj1", "user1")
	acl.SetOwner("obj2", "user2")

	acl.AddSuperUsers([]string{"super1"}, FullAccess)

	assert.True(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "obj1",
		Action:  FullAccess,
	}))

	assert.True(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "obj1",
		Action:  ReadAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "none",
		Action:  ReadAccess,
	}))
}

func TestSuperFull(t *testing.T) {
	acl := New()

	acl.SetOwner("obj1", "user1")
	acl.SetOwner("obj2", "user2")

	acl.AddSuperUsers([]string{"super1"}, ReadAccess)

	assert.False(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "obj1",
		Action:  FullAccess,
	}))

	assert.True(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "obj1",
		Action:  ReadAccess,
	}))

	assert.False(t, acl.Check(AccessRequest{
		Subject: "super1",
		Object:  "none",
		Action:  ReadAccess,
	}))
}
