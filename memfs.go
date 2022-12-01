package memfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	tempDir = "tmp"
)

type FS struct {
	root   *fsNode
	nextFD int64
	mutex  sync.Mutex
}

func New() *FS {
	f := new(FS)
	f.nextFD = 100

	f.root = &fsNode{
		name:     "",
		modified: time.Now(),
		perm:     fs.ModePerm,
		entries:  make(map[string]*fsNode),
	}
	f.root.entries[tempDir] = &fsNode{
		name:     tempDir,
		perm:     fs.ModePerm,
		modified: time.Now(),
		entries:  make(map[string]*fsNode),
	}

	cwd, _ := os.Getwd()
	_ = f.MkdirAll(cwd, fs.ModePerm)

	return f
}

func (f *FS) getNextFileDescriptor() int64 {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	fd := f.nextFD
	f.nextFD++
	return fd
}

func (f *FS) getAbsolutePath(path string) string {
	if !filepath.IsAbs(path) {
		path, _ = filepath.Abs(path)
	}
	return filepath.Clean(path)
}

func (f *FS) randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (f *FS) createRandomPathPart(pattern string) string {
	if !strings.Contains(pattern, "*") {
		pattern = pattern + "*"
	}
	return strings.Replace(pattern, "*", f.randomString(8), -1)
}

func (f *FS) ValidPath(path string) bool {
	if !utf8.ValidString(path) {
		return false
	}
	return true
}

func (f *FS) getEntry(path string) (parent *fsNode, entry *fsNode, missingPath string, err error) {
	if !f.ValidPath(path) {
		return nil, nil, "", fmt.Errorf("invalid path: %s: %w", path, fs.ErrInvalid)
	}

	path = f.getAbsolutePath(path)

	parentDir, lastEntry := filepath.Split(path)
	if parentDir == "/" && lastEntry == "" {
		// was requesting entry for root dir
		return f.root, nil, "", nil
	}

	var parts []string
	if parentDir == "/" {
		// handle root dir
		parts = []string{""}
	} else {
		parts = strings.Split(filepath.Clean(parentDir), string(filepath.Separator))
	}

	if len(parts) == 0 || parts[0] != "" {
		return nil, nil, "", fmt.Errorf("invalid path: %s: %w", path, fs.ErrInvalid)
	}

	current := f.root
	parts = parts[1:]
	for i, part := range parts {
		current.mutex.Lock()
		if e, exists := current.entries[part]; exists {
			if !e.isDir() {
				current.mutex.Unlock()
				return nil, nil, "", fmt.Errorf("not a directory: %s: %w", part, fs.ErrInvalid)
			}
			current.mutex.Unlock()
			current = e
		} else {
			current.mutex.Unlock()
			return current, nil, strings.Join(parts[i:], string(filepath.Separator)), nil
		}
	}

	if e, exists := current.entries[lastEntry]; exists {
		return current, e, "", nil
	}

	return current, nil, lastEntry, nil
}

func (f *FS) MkdirAll(path string, perm os.FileMode) error {
	if !f.ValidPath(path) {
		return fmt.Errorf("invalid path: %s: %w", path, fs.ErrInvalid)
	}

	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) == 0 || parts[0] != "" {
		return fmt.Errorf("invalid path: %s: %w", path, fs.ErrInvalid)
	}
	if len(parts) == 1 {
		return nil
	}

	current := f.root
	for _, part := range parts[1:] {
		current.mutex.Lock()
		if entry, exists := current.entries[part]; exists {
			if !entry.isDir() {
				current.mutex.Unlock()
				return fmt.Errorf("not a directory: %s: %w", part, fs.ErrInvalid)
			}
			current.mutex.Unlock()
			current = entry
		} else {
			entry := &fsNode{
				name:     part,
				perm:     perm,
				modified: time.Now(),
				entries:  make(map[string]*fsNode),
			}
			current.entries[part] = entry
			current.mutex.Unlock()
			current = entry
		}
	}
	return nil
}

type fileFlags int

func (f fileFlags) isSet(mask int) bool {
	if int(f)&mask == mask {
		return true
	}
	return false
}
func (f fileFlags) isReadOnly() bool {
	return int(f) == os.O_RDONLY
}
func (f fileFlags) isWriteOnly() bool {
	return f.isSet(os.O_WRONLY)
}
func (f fileFlags) isReadWrite() bool {
	return f.isSet(os.O_RDWR)
}
func (f fileFlags) canRead() bool {
	return f.isReadOnly() || f.isReadWrite()
}
func (f fileFlags) canWrite() bool {
	return f.isWriteOnly() || f.isReadWrite()
}
func (f fileFlags) isAppend() bool {
	return f.isSet(os.O_APPEND)
}
func (f fileFlags) isCreate() bool {
	return f.isSet(os.O_CREATE)
}
func (f fileFlags) isCreateMustNotExist() bool {
	return f.isSet(os.O_EXCL)
}
func (f fileFlags) isTruncating() bool {
	return f.isSet(os.O_TRUNC)
}

func (f *FS) Open(path string) (*File, error) {
	return f.OpenFile(path, os.O_RDONLY, 0)
}
func (f *FS) Create(path string) (*File, error) {
	return f.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}
func (f *FS) OpenFile(path string, flag int, perm os.FileMode) (*File, error) {
	fileFlag := fileFlags(flag)

	parentNode, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return nil, err
	}

	// the path yet to create would point to a further nesting directory, the full path to the parent
	// directory does not exist and should be an error
	if missingPath != "" && len(strings.Split(missingPath, string(filepath.Separator))) > 1 {
		return nil, fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}

	crws := &contentReadWriteSeekerImpl{owner: entryNode}

	if entryNode != nil {
		if entryNode.isDir() {
			return &File{
				node: entryNode,
				flag: fileFlag,
				fd:   f.getNextFileDescriptor(),
			}, nil
		}
		if fileFlag.canWrite() {
			if fileFlag.isCreate() && fileFlag.isCreateMustNotExist() {
				return nil, fmt.Errorf("path exists: %s: %w", path, fs.ErrExist)
			}
			if fileFlag.isTruncating() {
				entryNode.lockContent()
				entryNode.content = []byte{}
				entryNode.unlockContent()
			} else if fileFlag.isAppend() {
				_, _ = crws.Seek(0, io.SeekEnd)
			}
		}
	} else {
		if fileFlag.isReadOnly() {
			return nil, fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
		} else {
			if fileFlag.isCreate() {
				parentNode.mutex.Lock()
				defer parentNode.mutex.Unlock()
				entryNode = &fsNode{
					name:     missingPath,
					perm:     perm,
					modified: time.Now(),
					content:  []byte{},
				}
				crws.owner = entryNode
				parentNode.entries[missingPath] = entryNode
			} else {
				return nil, fmt.Errorf("path does not exist and cannot create: %s: %w", path, fs.ErrInvalid)
			}
		}
	}

	return &File{
		node: entryNode,
		flag: fileFlag,
		crws: crws,
		fd:   f.getNextFileDescriptor(),
	}, nil
}

func (f *FS) Stat(path string) (FileInfo, error) {
	_, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return FileInfo{}, err
	}
	if missingPath != "" {
		return FileInfo{}, fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}
	return FileInfo{node: entryNode}, nil
}

func (f *FS) Remove(path string) error {
	parentNode, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return err
	}
	if missingPath != "" {
		return fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}
	if entryNode.isDir() {
		if len(entryNode.entries) == 0 {
			parentNode.mutex.Lock()
			defer parentNode.mutex.Unlock()
			entryNode.unlinked = true
			delete(parentNode.entries, entryNode.name)
		} else {
			return fmt.Errorf("directory not empty: %s: %w", path, fs.ErrInvalid)
		}
	} else {
		parentNode.mutex.Lock()
		defer parentNode.mutex.Unlock()
		entryNode.unlinked = true
		delete(parentNode.entries, entryNode.name)
	}
	return nil
}

func (f *FS) RemoveAll(path string) error {
	parentNode, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return err
	}
	if missingPath != "" {
		return fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}
	if entryNode.isDir() {
		if len(entryNode.entries) == 0 {
			parentNode.mutex.Lock()
			defer parentNode.mutex.Unlock()
			entryNode.unlinked = true
			delete(parentNode.entries, entryNode.name)
			for part := range entryNode.entries {
				_ = f.RemoveAll(filepath.Join(path, entryNode.name, part))
			}
		} else {
			return fmt.Errorf("directory not empty: %s: %w", path, fs.ErrInvalid)
		}
	} else {
		parentNode.mutex.Lock()
		defer parentNode.mutex.Unlock()
		entryNode.unlinked = true
		delete(parentNode.entries, entryNode.name)
	}
	return nil
}
func (f *FS) ReadDir(path string) ([]os.DirEntry, error) {
	_, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return nil, err
	}
	if missingPath != "" {
		return nil, fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}
	names := entryNode.getEntryNames()
	entryNode.mutex.Lock()
	defer entryNode.mutex.Unlock()
	dirEntries := make([]os.DirEntry, len(names), len(names))
	for i := range names {
		dirEntries[i] = DirEntry{
			node: entryNode.entries[names[i]],
		}
	}
	return dirEntries, nil
}

func (f *FS) Mkdir(path string, perm os.FileMode) error {
	parentNode, entryNode, missingPath, err := f.getEntry(path)
	if err != nil {
		return err
	}
	if entryNode == nil && missingPath == "" {
		// this is a special case of trying to recreate the root dir
		return nil
	}
	if entryNode != nil {
		return fmt.Errorf("path exists: %s: %w", path, fs.ErrExist)
	}
	if missingPath != "" && len(strings.Split(missingPath, string(filepath.Separator))) > 1 {
		return fmt.Errorf("path does not exist: %s: %w", path, fs.ErrNotExist)
	}
	parentNode.mutex.Lock()
	defer parentNode.mutex.Unlock()
	entryNode = &fsNode{
		name:     missingPath,
		perm:     perm,
		modified: time.Now(),
		entries:  make(map[string]*fsNode),
	}
	parentNode.entries[missingPath] = entryNode
	return nil
}

func (f *FS) CreateTemp(dir, pattern string) (*File, error) {
	if dir == "" {
		dir = f.TempDir()
	}

	_, entryNode, _, err := f.getEntry(dir)
	if err != nil {
		return nil, err
	}
	if entryNode == nil || !entryNode.isDir() {
		return nil, fmt.Errorf("dir does not exist: %s: %w", dir, fs.ErrNotExist)
	}

	var file *File
	err = errors.New("tmp")
	for err != nil {
		file, err = f.Create(filepath.Join(dir, f.createRandomPathPart(pattern)))
	}
	return file, nil
}

func (f *FS) MkdirTemp(dir, pattern string) (name string, err error) {
	if dir == "" {
		dir = f.TempDir()
	}

	_, entryNode, _, err := f.getEntry(dir)
	if err != nil {
		return "", err
	}
	if entryNode == nil || !entryNode.isDir() {
		return "", fmt.Errorf("dir does not exist: %s: %w", dir, fs.ErrNotExist)
	}

	var tDir string
	err = errors.New("tmp")
	for err != nil {
		tDir = filepath.Join(dir, f.createRandomPathPart(pattern))
		err = f.Mkdir(tDir, fs.ModePerm)
	}

	return tDir, nil
}

func (f *FS) TempDir() string {
	return string(filepath.Separator) + tempDir
}
