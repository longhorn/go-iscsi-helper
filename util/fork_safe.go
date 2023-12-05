package util

import (
	"io"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func NsReadFile(name string) ([]byte, error) {
	nameRaw, err := unix.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}
	fd, err := open(nameRaw, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer close_raw(fd)

	data := make([]byte, 0, 4096)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := read(fd, data[len(data):cap(data)])
		if n == 0 {
			return data, nil
		}
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}

func NsWriteFile(name string, data []byte, perm os.FileMode) error {
	nameRaw, err := unix.BytePtrFromString(name)
	if err != nil {
		return err
	}
	fd, err := open(nameRaw, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, uint32(perm))
	if err != nil {
		return err
	}
	defer close_raw(fd)

	written := 0
	for written < len(data) {
		newly_written, err := write(fd, data[written:])
		if err != nil {
			return err
		}
		written += newly_written
	}

	return nil
}

type fileStat struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	sys     syscall.Stat_t
}

func (fs *fileStat) Size() int64        { return fs.size }
func (fs *fileStat) Mode() fs.FileMode  { return fs.mode }
func (fs *fileStat) ModTime() time.Time { return fs.modTime }
func (fs *fileStat) Sys() any           { return &fs.sys }
func (fs *fileStat) Name() string       { return fs.name }
func (fs *fileStat) IsDir() bool        { return fs.Mode().IsDir() }

func basename(name string) string {
	i := len(name) - 1
	// Remove trailing slashes
	for ; i > 0 && name[i] == '/'; i-- {
		name = name[:i]
	}
	// Remove leading directory name
	for i--; i >= 0; i-- {
		if name[i] == '/' {
			name = name[i+1:]
			break
		}
	}

	return name
}

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = basename(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtim.Unix())
	fs.mode = os.FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= os.ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= os.ModeDevice | os.ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= os.ModeDir
	case syscall.S_IFIFO:
		fs.mode |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= os.ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= os.ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= os.ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= os.ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= os.ModeSticky
	}
}

func ignoringEINTR(fn func() error) error {
	for {
		err := fn()
		if err != syscall.EINTR {
			return err
		}
	}
}
func ignoringEINTRIO(fn func(fd int, p []byte) (int, error), fd int, p []byte) (int, error) {
	for {
		n, err := fn(fd, p)
		if err != syscall.EINTR {
			return n, err
		}
	}
}

func fstatat(fd int, path string, stat *syscall.Stat_t, flags int) (err error) {
	var _p0 *byte
	_p0, err = unix.BytePtrFromString(path)
	if err != nil {
		return
	}
	r0, _, err := unix.RawSyscall6(unix.SYS_NEWFSTATAT, uintptr(fd), uintptr(unsafe.Pointer(_p0)), uintptr(unsafe.Pointer(stat)), uintptr(flags), 0, 0)
	if r0 == 0 {
		err = nil
	}
	return
}

const (
	_AT_FDCWD            = -0x64
	_AT_REMOVEDIR        = 0x200
	_AT_SYMLINK_NOFOLLOW = 0x100
)

func NsStat(name string) (fs.FileInfo, error) {
	var fs fileStat
	err := ignoringEINTR(func() error {
		return fstatat(_AT_FDCWD, name, &fs.sys, 0)
	})
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	fillFileStatFromSys(&fs, name)
	return &fs, nil
}

func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}

func NsMkdir(name string, perm os.FileMode) error {
	e := ignoringEINTR(func() error {
		return syscall.Mkdir(name, syscallMode(perm))
	})

	if e != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: e}
	}

	return nil
}

func NsLstat(name string) (os.FileInfo, error) {
	var fs fileStat
	err := ignoringEINTR(func() error {
		return fstatat(_AT_FDCWD, name, &fs.sys, _AT_SYMLINK_NOFOLLOW)
	})
	if err != nil {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	fillFileStatFromSys(&fs, name)
	return &fs, nil
}

func NsMkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := NsStat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = NsMkdirAll(path[:j-1], perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = NsMkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := NsLstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

func unlinkat(dirfd int, path string, flags int) (err error) {
	var _p0 *byte
	_p0, err = unix.BytePtrFromString(path)
	if err != nil {
		return
	}
	r0, _, err := unix.RawSyscall(unix.SYS_UNLINKAT, uintptr(dirfd), uintptr(unsafe.Pointer(_p0)), uintptr(flags))
	if r0 == 0 {
		err = nil
	}
	return
}

func NsRemove(name string) error {
	// System call interface forces us to know
	// whether name is a file or directory.
	// Try both: it is cheaper on average than
	// doing a Stat plus the right one.
	e := ignoringEINTR(func() error {
		return unlinkat(_AT_FDCWD, name, 0)
	})
	if e == nil {
		return nil
	}
	e1 := ignoringEINTR(func() error {
		return unlinkat(_AT_FDCWD, name, _AT_REMOVEDIR)
	})
	if e1 == nil {
		return nil
	}

	// Both failed: figure out which error to return.
	// OS X and Linux differ on whether unlink(dir)
	// returns EISDIR, so can't use that. However,
	// both agree that rmdir(file) returns ENOTDIR,
	// so we can use that to decide which error is real.
	// Rmdir might also return ENOTDIR if given a bad
	// file path, like /etc/passwd/foo, but in that case,
	// both errors will be ENOTDIR, so it's okay to
	// use the error from unlink.
	if e1 != syscall.ENOTDIR {
		e = e1
	}
	return &os.PathError{Op: "remove", Path: name, Err: e}
}

func NsReadDir(name string) ([]os.DirEntry, error) {
	nameRaw, err := unix.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}

	fd, err := open(nameRaw, unix.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer close_raw(fd)

	dirs, err := readDir(name, fd, -1)
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	return dirs, err
}

func readDir(name string, fd int, n int) ([]os.DirEntry, error) {
	_, dirents, _, err := readdir(name, fd, n)
	if dirents == nil {
		// Match Readdir and Readdirnames: don't return nil slices.
		dirents = []os.DirEntry{}
	}
	return dirents, err
}

func readDirentInterior(fd int, buf []byte) (n int, err error) {
	var _p0 unsafe.Pointer
	if len(buf) > 0 {
		_p0 = unsafe.Pointer(&buf[0])
	} else {
		_p0 = unsafe.Pointer(&_zero)
	}
	r0, _, err := unix.RawSyscall(unix.SYS_GETDENTS64, uintptr(fd), uintptr(_p0), uintptr(len(buf)))
	if int(r0) == -1 {
		n = int(r0)
	} else {
		err = nil
	}
	return
}

func readDirent(fd int, buf []byte) (int, error) {
	n, err := ignoringEINTRIO(readDirentInterior, fd, buf)
	if err != nil {
		n = 0
	}
	return n, err
}

type dirInfo struct {
	buf  *[]byte // buffer for directory I/O
	nbuf int     // length of buf; return value from Getdirentries
	bufp int     // location of next record in buf.
}

func readInt(b []byte, off, size uintptr) (u uint64, ok bool) {
	if len(b) < int(off+size) {
		return 0, false
	}
	return readIntLE(b[off:], size), true
}

func readIntLE(b []byte, size uintptr) uint64 {
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		_ = b[1] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8
	case 4:
		_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24
	case 8:
		_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
		return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
			uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	default:
		panic("syscall: readInt with unsupported size")
	}
}

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	reclen, ok := direntReclen(buf)
	if !ok {
		return 0, false
	}
	return reclen - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)), true
}

func direntType(buf []byte) os.FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^os.FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return os.ModeDevice
	case syscall.DT_CHR:
		return os.ModeDevice | os.ModeCharDevice
	case syscall.DT_DIR:
		return os.ModeDir
	case syscall.DT_FIFO:
		return os.ModeNamedPipe
	case syscall.DT_LNK:
		return os.ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return os.ModeSocket
	}
	return ^os.FileMode(0) // unknown
}

type unixDirent struct {
	parent string
	name   string
	typ    os.FileMode
	info   os.FileInfo
}

func (d *unixDirent) Name() string      { return d.name }
func (d *unixDirent) IsDir() bool       { return d.typ.IsDir() }
func (d *unixDirent) Type() os.FileMode { return d.typ }

func (d *unixDirent) Info() (os.FileInfo, error) {
	if d.info != nil {
		return d.info, nil
	}
	return NsLstat(d.parent + "/" + d.name)
}

func (d *unixDirent) String() string {
	return fs.FormatDirEntry(d)
}

func newUnixDirent(parent, name string, typ os.FileMode) (os.DirEntry, error) {
	ude := &unixDirent{
		parent: parent,
		name:   name,
		typ:    typ,
	}
	if typ != ^os.FileMode(0) {
		return ude, nil
	}

	info, err := NsLstat(parent + "/" + name)
	if err != nil {
		return nil, err
	}

	ude.typ = info.Mode().Type()
	ude.info = info
	return ude, nil
}

func readdir(base_name string, fd int, n int) (names []string, dirents []os.DirEntry, infos []os.FileInfo, err error) {
	d := new(dirInfo)
	d.buf = &[]byte{}

	// Change the meaning of n for the implementation below.
	//
	// The n above was for the public interface of "if n <= 0,
	// Readdir returns all the FileInfo from the directory in a
	// single slice".
	//
	// But below, we use only negative to mean looping until the
	// end and positive to mean bounded, with positive
	// terminating at 0.
	if n == 0 {
		n = -1
	}

	for n != 0 {
		// Refill the buffer if necessary
		if d.bufp >= d.nbuf {
			d.bufp = 0
			var errno error
			d.nbuf, errno = readDirent(fd, *d.buf)
			if errno != nil {
				return names, dirents, infos, &os.PathError{Op: "readdirent", Path: base_name, Err: errno}
			}
			if d.nbuf <= 0 {
				break // EOF
			}
		}

		// Drain the buffer
		buf := (*d.buf)[d.bufp:d.nbuf]
		reclen, ok := direntReclen(buf)
		if !ok || reclen > uint64(len(buf)) {
			break
		}
		rec := buf[:reclen]
		d.bufp += int(reclen)
		ino, ok := direntIno(rec)
		if !ok {
			break
		}
		// When building to wasip1, the host runtime might be running on Windows
		// or might expose a remote file system which does not have the concept
		// of inodes. Therefore, we cannot make the assumption that it is safe
		// to skip entries with zero inodes.
		if ino == 0 && runtime.GOOS != "wasip1" {
			continue
		}
		const namoff = uint64(unsafe.Offsetof(syscall.Dirent{}.Name))
		namlen, ok := direntNamlen(rec)
		if !ok || namoff+namlen > uint64(len(rec)) {
			break
		}
		name := rec[namoff : namoff+namlen]
		for i, c := range name {
			if c == 0 {
				name = name[:i]
				break
			}
		}
		// Check for useless names before allocating a string.
		if string(name) == "." || string(name) == ".." {
			continue
		}
		if n > 0 { // see 'n == 0' comment above
			n--
		}
		de, err := newUnixDirent(base_name, string(name), direntType(rec))
		if os.IsNotExist(err) {
			// File disappeared between readdir and stat.
			// Treat as if it didn't exist.
			continue
		}
		if err != nil {
			return nil, dirents, nil, err
		}
		dirents = append(dirents, de)
	}

	if n > 0 && len(names)+len(dirents)+len(infos) == 0 {
		return nil, nil, nil, io.EOF
	}
	return names, dirents, infos, nil
}

func endsWithDot(path string) bool {
	if path == "." {
		return true
	}
	if len(path) >= 2 && path[len(path)-1] == '.' && os.IsPathSeparator(path[len(path)-2]) {
		return true
	}
	return false
}

func splitPath(path string) (string, string) {
	// if no better parent is found, the path is relative from "here"
	dirname := "."

	// Remove all but one leading slash.
	for len(path) > 1 && path[0] == '/' && path[1] == '/' {
		path = path[1:]
	}

	i := len(path) - 1

	// Remove trailing slashes.
	for ; i > 0 && path[i] == '/'; i-- {
		path = path[:i]
	}

	// if no slashes in path, base is path
	basename := path

	// Remove leading directory path
	for i--; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				dirname = path[:1]
			} else {
				dirname = path[:i]
			}
			basename = path[i+1:]
			break
		}
	}

	return dirname, basename
}

func NsRemoveAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return nil
	}

	// The rmdir system call does not permit removing ".",
	// so we don't permit it either.
	if endsWithDot(path) {
		return &os.PathError{Op: "RemoveAll", Path: path, Err: syscall.EINVAL}
	}

	// Simple case: if Remove works, we're done.
	err := NsRemove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// RemoveAll recurses by deleting the path base from
	// its parent directory
	parentDir, base := splitPath(path)

	parentDirRaw, err := unix.BytePtrFromString(parentDir)
	if err != nil {
		return err
	}

	parentFd, err := open(parentDirRaw, unix.O_RDONLY, 0644)

	if os.IsNotExist(err) {
		// If parent does not exist, base cannot exist. Fail silently
		return nil
	}
	if err != nil {
		return err
	}
	defer close_raw(parentFd)

	if err := removeAllFrom(parentFd, base); err != nil {
		if pathErr, ok := err.(*os.PathError); ok {
			pathErr.Path = parentDir + string(os.PathSeparator) + pathErr.Path
			err = pathErr
		}
		return err
	}
	return nil
}

func removeAllFrom(parentFd int, base string) error {
	// Simple case: if Unlink (aka remove) works, we're done.
	err := ignoringEINTR(func() error {
		return unix.Unlinkat(parentFd, base, 0)
	})
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// EISDIR means that we have a directory, and we need to
	// remove its contents.
	// EPERM or EACCES means that we don't have write permission on
	// the parent directory, but this entry might still be a directory
	// whose contents need to be removed.
	// Otherwise just return the error.
	if err != syscall.EISDIR && err != syscall.EPERM && err != syscall.EACCES {
		return &os.PathError{Op: "unlinkat", Path: base, Err: err}
	}

	// Is this a directory we need to recurse into?
	var statInfo unix.Stat_t
	statErr := ignoringEINTR(func() error {
		return unix.Fstatat(parentFd, base, &statInfo, unix.AT_SYMLINK_NOFOLLOW)
	})
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return &os.PathError{Op: "fstatat", Path: base, Err: statErr}
	}
	if statInfo.Mode&syscall.S_IFMT != syscall.S_IFDIR {
		// Not a directory; return the error from the unix.Unlinkat.
		return &os.PathError{Op: "unlinkat", Path: base, Err: err}
	}

	// Remove the directory's entries.
	var recurseErr error
	for {
		const reqSize = 1024
		var respSize int

		// Open the directory to recurse into
		file, err := openFdAt(parentFd, base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			recurseErr = &os.PathError{Op: "openfdat", Path: base, Err: err}
			break
		}
		defer close_raw(file)

		for {
			numErr := 0

			names, readErr := readDir(base, file, -1)
			// Errors other than EOF should stop us from continuing.
			if readErr != nil && readErr != io.EOF {
				if os.IsNotExist(readErr) {
					return nil
				}
				return &os.PathError{Op: "readdirnames", Path: base, Err: readErr}
			}

			respSize = len(names)
			for _, name := range names {
				err := removeAllFrom(file, name.Name())
				if err != nil {
					if pathErr, ok := err.(*os.PathError); ok {
						pathErr.Path = base + string(os.PathSeparator) + pathErr.Path
					}
					numErr++
					if recurseErr == nil {
						recurseErr = err
					}
				}
			}

			// If we can delete any entry, break to start new iteration.
			// Otherwise, we discard current names, get next entries and try deleting them.
			if numErr != reqSize {
				break
			}
		}

		// Finish when the end of the directory is reached
		if respSize < reqSize {
			break
		}
	}

	// Remove the directory itself.
	unlinkError := ignoringEINTR(func() error {
		return unix.Unlinkat(parentFd, base, unix.AT_REMOVEDIR)
	})
	if unlinkError == nil || os.IsNotExist(unlinkError) {
		return nil
	}

	if recurseErr != nil {
		return recurseErr
	}
	return &os.PathError{Op: "unlinkat", Path: base, Err: unlinkError}
}

func openFdAt(dirfd int, name string) (int, error) {
	var r int
	for {
		var e error
		r, e = unix.Openat(dirfd, name, os.O_RDONLY|syscall.O_CLOEXEC, 0)
		if e == nil {
			break
		}

		// See comment in openFileNolog.
		if e == syscall.EINTR {
			continue
		}

		return -1, e
	}

	// if !supportsCloseOnExec {
	// 	syscall.CloseOnExec(r)
	// }

	return r, nil
}
