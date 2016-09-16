package initiator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/convoy/util"
)

const (
	iscsiBinary = "iscsiadm"
)

func LoginTarget(ip, target string) error {
	opts := []string{
		"-m", "node",
		"-T", target,
		"-p", ip,
		"--login",
	}
	_, err := util.Execute(iscsiBinary, opts)
	if err != nil {
		return err
	}
	return nil
}

func GetDevice(ip, target string) (string, error) {
	path := "/dev/disk/by-path/ip-" + ip + ":3260-iscsi-" + target + "-lun-0"
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("Cannot find device for %v and %v: %v",
			ip, target, err)
	}
	dev, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return dev, nil
}

func LogoutTarget(ip, target string) error {
	opts := []string{
		"-m", "node",
		"-T", target,
		"-p", ip,
		"--logout",
	}
	_, err := util.Execute(iscsiBinary, opts)
	if err != nil {
		return err
	}
	return nil
}
