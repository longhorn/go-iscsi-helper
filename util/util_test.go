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
	ip, err := GetIPToHost()
	c.Assert(err, IsNil)
	c.Assert(ip, NotNil)
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

func (s *TestSuite) TestFindDockerdProcess(c *C) {
	procPath := "/host/proc"
	finder := NewProcessFinder(procPath)

	ps, err := finder.FindAncestorByName(DockerdProcess)
	if err != nil {
		ps, err = finder.FindAncestorByName(ContainerdProcess)
		if err != nil {
			ps, err = finder.FindAncestorByName(ContainerdProcessShim)
		}
	}
	c.Assert(err, IsNil)
	c.Assert(ps, NotNil)
	c.Assert(fmt.Sprintf("%s/%d/ns/", procPath, ps.Pid), Equals, GetHostNamespacePath(procPath))

	notExistProcess := "dockerdddd"
	ps, err = finder.FindAncestorByName(notExistProcess)
	c.Assert(err, NotNil)
	c.Assert(ps, IsNil)
}
