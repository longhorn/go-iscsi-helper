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
