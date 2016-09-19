package util

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
}

var _ = Suite(&TestSuite{})

func (s *TestSuite) TestGetLocalIPs(c *C) {
	ips, err := GetLocalIPs()
	c.Assert(err, IsNil)
	c.Assert(ips, NotNil)
}

func (s *TestSuite) TestNamespaceExecutor(c *C) {
	var err error

	ne, err := NewNamespaceExecutor("")
	c.Assert(err, IsNil)
	_, err = ne.Execute("ls", []string{})
	c.Assert(err, IsNil)
	_, err = ne.Execute("mount", []string{})
	c.Assert(err, IsNil)

	ne, err = NewNamespaceExecutor("/host/proc/1/ns")
	c.Assert(err, IsNil)
	_, err = ne.Execute("ls", []string{})
	c.Assert(err, IsNil)
	_, err = ne.Execute("mount", []string{})
	c.Assert(err, IsNil)
}
