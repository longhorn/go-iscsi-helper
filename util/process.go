package util

import (
	"fmt"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

const (
	DockerdProcess    = "dockerd"
	ContainerdProcess = "containerd"
)

type ProcessFinder struct {
	procPath string
}

func NewProcessFinder(procPath string) *ProcessFinder {
	return &ProcessFinder{procPath}
}

func (p *ProcessFinder) FindPid(pid int64) (*linuxproc.ProcessStatus, error) {
	path := fmt.Sprintf("%s/%d/status", p.procPath, pid)
	return linuxproc.ReadProcessStatus(path)
}

func (p *ProcessFinder) FindSelf() (*linuxproc.ProcessStatus, error) {
	path := fmt.Sprintf("%s/self/status", p.procPath)
	return linuxproc.ReadProcessStatus(path)
}

func (p *ProcessFinder) FindAncestorByName(ancestorProcess string) (*linuxproc.ProcessStatus, error) {
	ps, err := p.FindSelf()
	if err != nil {
		return nil, err
	}

	for {
		if ps.Name == ancestorProcess {
			return ps, nil
		}
		if ps.PPid == 0 {
			break
		}
		ps, err = p.FindPid(ps.PPid)
		if err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("Failed to find the ancestor process: %s", ancestorProcess)
}

func GetHostNamespacePath(hostProcPath string) (string, error) {
	pf := NewProcessFinder(hostProcPath)
	var proc *linuxproc.ProcessStatus
	var err1, err2 error
	proc, err1 = pf.FindAncestorByName(DockerdProcess)
	if err1 != nil {
		proc, err2 = pf.FindAncestorByName(ContainerdProcess)
		if err2 != nil {
			return fmt.Sprintf("%s/%d/ns/", hostProcPath, 1), fmt.Errorf("failed to find proc dockerd or containerd, fall back to use pid 1 for host namespace path: %v, %v", err1, err2)
		}
	}
	return fmt.Sprintf("%s/%d/ns/", hostProcPath, proc.Pid), nil
}
