package memfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"sync"
	"time"
)

type fsNode struct {
	name     string
	perm     os.FileMode
	modified time.Time
	content  []byte
	mutex    sync.Mutex
	entries  map[string]*fsNode
	unlinked bool
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
	buf               *bytes.Buffer
	fd                int64
	pos               int64
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
	if f.node.unlinked || !f.flag.canRead() {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	n, err = f.buf.Read(p)
	f.pos += int64(n)
	return
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f.node.unlinked || !f.flag.canRead() {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	return bytes.NewBuffer(f.node.content[off:]).Read(p)
}

func (f *File) Seek(offset int64, whence int) (n int64, err error) {
	if f.node.unlinked {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	newPos, offs := int64(0), offset
	switch whence {
	case io.SeekStart:
		newPos = offs
	case io.SeekCurrent:
		newPos = f.pos + offs
	case io.SeekEnd:
		newPos = int64(f.buf.Len()) + offs
	}
	if newPos < 0 {
		return 0, errors.New("negative result pos")
	}
	f.pos = newPos
	f.buf = bytes.NewBuffer(f.node.content[f.pos:])
	return newPos, nil
}

func (f *File) Write(p []byte) (n int, err error) {
	if f.node.unlinked || !f.flag.canWrite() {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	// if the offset is past the end of the buffer, grow the buffer with null bytes.
	if extra := f.pos - int64(f.buf.Len()); extra > 0 {
		if _, err := f.buf.Write(make([]byte, extra)); err != nil {
			return n, err
		}
	}
	// if the offset isn't at the end of the buffer, write as much as we can.
	if f.pos < int64(f.buf.Len()) {
		n = copy(f.buf.Bytes()[f.pos:], p)
		p = p[n:]
	}
	// if there are remaining bytes, append them to the buffer.
	if len(p) > 0 {
		var bn int
		bn, err = f.buf.Write(p)
		n += bn
	}

	f.node.content = f.buf.Bytes()

	f.pos += int64(n)
	return n, err
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	if f.node.unlinked || !f.flag.canWrite() {
		return 0, fmt.Errorf("file unlinked: %s: %w", f.Name(), fs.ErrInvalid)
	}
	if f.closed {
		return 0, fmt.Errorf("file closed: %s: %w", f.Name(), fs.ErrClosed)
	}
	f.node.mutex.Lock()
	defer f.node.mutex.Unlock()
	// if the offset isn't at the end of the buffer, write as much as we can.
	if off < int64(f.buf.Len()) {
		n = copy(f.buf.Bytes()[off:], p)
		p = p[n:]
	}
	// if there are remaining bytes, append them to the buffer.
	if len(p) > 0 {
		var bn int
		bn, err = f.buf.Write(p)
		n += bn
	}

	f.node.content = f.buf.Bytes()
	return n, err
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
	if n < 0 || n < len(names) {
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
	if n < 0 || n < len(names) {
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
	if n < 0 || n < len(names) {
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
