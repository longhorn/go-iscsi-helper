package target

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/yasker/go-iscsi-helper/util"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
	imageFile string
	localIP   string
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
}

func (s *TestSuite) TearDownSuite(c *C) {
	err := exec.Command("rm", "-rf", testRoot).Run()
	c.Assert(err, IsNil)
}

func (s *TestSuite) TestDaemonBasic(c *C) {
	var err error

	err = StartDaemon()
	c.Assert(err, IsNil)

	err = CreateTarget(1, "iqn.2016-09.com.rancher:for.all")
	c.Assert(err, IsNil)

	err = AddLunBackedByFile(1, 1, s.imageFile)
	c.Assert(err, IsNil)

	err = BindInitiator(1, "ALL")
	c.Assert(err, IsNil)

	err = UnbindInitiator(1, "ALL")
	c.Assert(err, IsNil)

	err = DeleteLun(1, 1)
	c.Assert(err, IsNil)

	err = DeleteTarget(1)
	c.Assert(err, IsNil)
}
