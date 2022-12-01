package memfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"sync"
	"time"
)

type contentOwner interface {
	lockContent()
	unlockContent()
	getContent() []byte
	setContent(c []byte)
}

type contentReadWriteSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
}

type contentReadWriteSeekerImpl struct {
	owner contentOwner
	pos   int
}

func (crws *contentReadWriteSeekerImpl) read(p []byte) (n int, err error) {
	content := crws.owner.getContent()
	if crws.pos >= len(p) {
		return 0, io.EOF
	}
	n = copy(p, content[crws.pos:])
	crws.pos += n
	return n, nil
}

func (crws *contentReadWriteSeekerImpl) Read(p []byte) (n int, err error) {
	crws.owner.lockContent()
	defer crws.owner.unlockContent()
	return crws.read(p)
}

func (crws *contentReadWriteSeekerImpl) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, os.ErrInvalid
	}
	crws.owner.lockContent()
	defer crws.owner.unlockContent()
	crws.pos = int(off)
	return crws.read(p)
}

func (crws *contentReadWriteSeekerImpl) Seek(offset int64, whence int) (int64, error) {
	crws.owner.lockContent()
	defer crws.owner.unlockContent()

	content := crws.owner.getContent()

	newPos, offs := 0, int(offset)
	switch whence {
	case io.SeekStart:
		newPos = offs
	case io.SeekCurrent:
		newPos = crws.pos + offs
	case io.SeekEnd:
		newPos = len(content) + offs
	}
	if newPos < 0 {
		return 0, os.ErrInvalid
	}

	crws.pos = newPos
	return int64(newPos), nil
}

func (crws *contentReadWriteSeekerImpl) write(p []byte) (n int, err error) {
	content := crws.owner.getContent()

	var newContent []byte

	if crws.pos >= len(content) {
		l := crws.pos + len(p)
		newContent = make([]byte, l, l)
		copy(newContent, content)
	} else if crws.pos+len(p) > len(content) {
		l := crws.pos + len(p)
		newContent = make([]byte, l, l)
		copy(newContent, content)
	} else {
		newContent = content
	}

	copy(newContent[crws.pos:], p)

	crws.owner.setContent(newContent)

	crws.pos += len(p)
	return len(p), nil
}

func (crws *contentReadWriteSeekerImpl) Write(p []byte) (n int, err error) {
	crws.owner.lockContent()
	defer crws.owner.unlockContent()
	return crws.write(p)
}

func (crws *contentReadWriteSeekerImpl) WriteAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, os.ErrInvalid
	}
	crws.owner.lockContent()
	defer crws.owner.unlockContent()
	crws.pos = int(off)
	return crws.write(p)
}

type fsNode struct {
	name     string
	perm     os.FileMode
	modified time.Time
	content  []byte
	mutex    sync.Mutex
	entries  map[string]*fsNode
	unlinked bool
}

func (f *fsNode) lockContent() {
	f.mutex.Lock()
}

func (f *fsNode) unlockContent() {
	f.mutex.Unlock()
}

func (f *fsNode) getContent() []byte {
	return f.content
}

func (f *fsNode) setContent(c []byte) {
	f.content = c
}

func (f *fsNode) isDir() bool {
	if f.entries != nil {
		return true
	}
	return false
}

func (f *fsNode) getEntryNames() []string {
	if f.isDir() {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		names := make([]string, 0, len(f.entries))
		for n := range f.entries {
			names = append(names, n)
		}
		sort.Strings(names)
		return names
	}
	return []string{}
}

type File struct {
	node              *fsNode
	flag              fileFlags
	fd                int64
	crws              *contentReadWriteSeekerImpl
	closed            bool
	indexReadDir      int
	indexReaddir      int
	indexReaddirnames int
}

func (f *File) isDir() bool {
	return f.node.isDir()
}

func (f *File) Name() string {
	return f.node.name
}

func (f *File) Stat() (os.FileInfo, error) {
	if f.node.unlinked {
		return FileInfo{}, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	return FileInfo{node: f.node}, nil
}

func (f *File) Close() error {
	if f.closed {
		return fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.closed = true
	return nil
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if !f.flag.canRead() {
		return 0, fmt.Errorf("cannot read: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	return f.crws.Read(p)
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if !f.flag.canRead() {
		return 0, fmt.Errorf("cannot read: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	return f.crws.ReadAt(p, off)
}

func (f *File) Seek(offset int64, whence int) (n int64, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	return f.crws.Seek(offset, whence)
}

func (f *File) Write(p []byte) (n int, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if !f.flag.canWrite() {
		return 0, fmt.Errorf("cannot write: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	return f.crws.Write(p)
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if !f.flag.canWrite() {
		return 0, fmt.Errorf("cannot write: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.flag.isAppend() {
		return 0, fmt.Errorf("append only file %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	return f.crws.WriteAt(p, off)
}

func (f *File) ReadDir(n int) ([]os.DirEntry, error) {
	if f.node.unlinked {
		return nil, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return nil, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	if !f.node.isDir() {
		return nil, fmt.Errorf("not a directoru: %s: %w", f.node.name, fs.ErrInvalid)
	}
	names := f.node.getEntryNames()
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	dirEntries := make([]os.DirEntry, len(names), len(names))
	for i := range names {
		dirEntries[i] = DirEntry{
			node: f.node.entries[names[i]],
		}
	}
	if n < 0 || n >= len(names) {
		return dirEntries, nil
	}
	dirEntries = dirEntries[f.indexReadDir:]
	if n < len(dirEntries) {
		f.indexReadDir += n
		return dirEntries[0:n], nil
	}
	f.indexReadDir = 0
	return dirEntries, nil
}

func (f *File) Readdir(n int) ([]os.FileInfo, error) {
	if f.node.unlinked {
		return nil, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return nil, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	if !f.node.isDir() {
		return nil, fmt.Errorf("not a directoru: %s: %w", f.node.name, fs.ErrInvalid)
	}
	names := f.node.getEntryNames()
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	fileInfos := make([]os.FileInfo, len(names), len(names))
	for i := range names {
		fileInfos[i] = FileInfo{
			node: f.node.entries[names[i]],
		}
	}
	if n < 0 || n >= len(names) {
		return fileInfos, nil
	}
	fileInfos = fileInfos[f.indexReaddir:]
	if n < len(fileInfos) {
		f.indexReaddir += n
		return fileInfos[0:n], nil
	}
	f.indexReaddir = 0
	return fileInfos, nil
}
func (f *File) Readdirnames(n int) ([]string, error) {
	if f.node.unlinked {
		return nil, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return nil, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	if !f.node.isDir() {
		return nil, fmt.Errorf("not a directoru: %s: %w", f.node.name, fs.ErrInvalid)
	}
	names := f.node.getEntryNames()
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	if n < 0 || n >= len(names) {
		return names, nil
	}
	names = names[f.indexReaddirnames:]
	if n < len(names) {
		f.indexReaddirnames += n
		return names[0:n], nil
	}
	f.indexReaddirnames = 0
	return names, nil
}
