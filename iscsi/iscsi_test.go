package iscsi

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yasker/go-iscsi-helper/util"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
	imageFile string
	localIP   string
	ne        *util.NamespaceExecutor
}

var _ = Suite(&TestSuite{})

const (
	testRoot  = "/tmp/target"
	testImage = "test.img"
	imageSize = 4 * 1024 * 1024 // 4M
)

func (s *TestSuite) createFile(file string, size int64) error {
	return exec.Command("truncate", "-s", strconv.FormatInt(size, 10), file).Run()
}

func (s *TestSuite) SetUpSuite(c *C) {
	err := exec.Command("mkdir", "-p", testRoot).Run()
	c.Assert(err, IsNil)

	s.imageFile = filepath.Join(testRoot, testImage)
	err = s.createFile(s.imageFile, imageSize)
	c.Assert(err, IsNil)

	err = exec.Command("mkfs.ext4", "-F", s.imageFile).Run()
	c.Assert(err, IsNil)

	ips, err := util.GetLocalIPs()
	c.Assert(err, IsNil)
	c.Assert(len(ips), Equals, 1)
	s.localIP = ips[0]

	s.ne, err = util.NewNamespaceExecutor("/host/proc/1/ns/")
	c.Assert(err, IsNil)

	err = StartDaemon(false)
	c.Assert(err, IsNil)

	err = StartDaemon(false)
	c.Assert(err, IsNil)
}

func (s *TestSuite) TearDownSuite(c *C) {
	err := exec.Command("rm", "-rf", testRoot).Run()
	c.Assert(err, IsNil)
}

func (s *TestSuite) TestFlow(c *C) {
	var (
		err    error
		exists bool
	)

	t := "iqn.2014-09.com.rancher:flow"
	tid := 1
	lun := 1
	tmptid := -1

	err = CheckForInitiatorExistence(s.ne)
	c.Assert(err, IsNil)

	tmptid, err = GetTargetTid(t)
	c.Assert(err, IsNil)
	c.Assert(tmptid, Equals, -1)

	err = CreateTarget(tid, t)
	c.Assert(err, IsNil)

	err = AddLunBackedByFile(tid, lun, s.imageFile)
	c.Assert(err, IsNil)

	err = BindInitiator(tid, "ALL")
	c.Assert(err, IsNil)

	tmptid, err = GetTargetTid(t)
	c.Assert(err, IsNil)
	c.Assert(tmptid, Equals, 1)

	exists = IsTargetLoggedIn(s.localIP, t, s.ne)
	c.Assert(exists, Equals, false)

	err = DiscoverTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = DeleteDiscoveredTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = DeleteDiscoveredTarget(s.localIP, t, s.ne)
	c.Assert(err, NotNil)

	err = LoginTarget(s.localIP, t, s.ne)
	c.Assert(err, NotNil)

	err = DiscoverTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = LoginTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	exists = IsTargetLoggedIn(s.localIP, t, s.ne)
	c.Assert(exists, Equals, true)

	dev, err := GetDevice(s.localIP, t, lun, s.ne)
	c.Assert(err, IsNil)
	c.Assert(strings.HasPrefix(dev, "/dev/sd"), Equals, true)

	err = LogoutTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	exists = IsTargetLoggedIn(s.localIP, t, s.ne)
	c.Assert(exists, Equals, false)

	err = DeleteDiscoveredTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = UnbindInitiator(tid, "ALL")
	c.Assert(err, IsNil)

	err = DeleteLun(tid, lun)
	c.Assert(err, IsNil)

	err = DeleteTarget(tid)
	c.Assert(err, IsNil)

	tmptid, err = GetTargetTid(t)
	c.Assert(err, IsNil)
	c.Assert(tmptid, Equals, -1)

}

func (s *TestSuite) TestAio(c *C) {
	var err error

	t := "iqn.2014-09.com.rancher:aio"
	tid := 1
	lun := 1

	err = CheckForInitiatorExistence(s.ne)
	c.Assert(err, IsNil)

	err = CreateTarget(tid, t)
	c.Assert(err, IsNil)

	err = AddLun(tid, lun, s.imageFile, "aio", "")
	c.Assert(err, IsNil)

	err = BindInitiator(tid, "ALL")
	c.Assert(err, IsNil)

	err = DiscoverTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = LoginTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	dev, err := GetDevice(s.localIP, t, lun, s.ne)
	c.Assert(err, IsNil)
	c.Assert(strings.HasPrefix(dev, "/dev/sd"), Equals, true)

	err = LogoutTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = DeleteDiscoveredTarget(s.localIP, t, s.ne)
	c.Assert(err, IsNil)

	err = UnbindInitiator(tid, "ALL")
	c.Assert(err, IsNil)

	err = DeleteLun(tid, lun)
	c.Assert(err, IsNil)

	err = DeleteTarget(tid)
	c.Assert(err, IsNil)
}
