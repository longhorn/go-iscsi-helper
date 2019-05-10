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

func (p *ProcessFinder) FindHostNamespacePID() (uint64, error) {
	var proc *linuxproc.ProcessStatus
	var err1, err2 error
	proc, err1 = p.FindAncestorByName(DockerdProcess)
	if err1 != nil {
		proc, err2 = p.FindAncestorByName(ContainerdProcess)
		if err2 != nil {
			return 1, fmt.Errorf("failed to find proc dockerd or containerd the ancestor process: %v, %v", err1, err2)
		}
	}
	return proc.Pid, nil
}
