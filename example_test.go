package memfs_test

import (
	"bufio"
	"errors"
	"github.com/mbordner/memfs"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
	Stat() (os.FileInfo, error)
	Name() string
	ReadDir(n int) ([]os.DirEntry, error)
	Readdir(count int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
}

type FS interface {
	Open(name string) (File, error)
	Create(name string) (File, error)
	Stat(name string) (os.FileInfo, error)
	Remove(name string) error
	CreateTemp(dir, pattern string) (File, error)
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	ReadDir(name string) ([]os.DirEntry, error)
	MkdirTemp(dir, pattern string) (name string, err error)
	TempDir() string
}

// osFS implements FS using the local disk.
type osFS struct{}

func (osFS) Open(name string) (File, error)                { return os.Open(name) }
func (osFS) Create(name string) (File, error)              { return os.Create(name) }
func (osFS) Stat(name string) (os.FileInfo, error)         { return os.Stat(name) }
func (osFS) Remove(name string) error                      { return os.Remove(name) }
func (osFS) CreateTemp(dir, pattern string) (File, error)  { return os.CreateTemp(dir, pattern) }
func (osFS) MkdirAll(path string, perms os.FileMode) error { return os.MkdirAll(path, perms) }
func (osFS) RemoveAll(path string) error                   { return os.RemoveAll(path) }
func (osFS) ReadDir(name string) ([]os.DirEntry, error)    { return os.ReadDir(name) }
func (osFS) MkdirTemp(dir, pattern string) (name string, err error) {
	return os.MkdirTemp(dir, pattern)
}
func (osFS) TempDir() string { return os.TempDir() }

// memFS implements FS using an in memory filesystem
type memFS struct{ fs *memfs.FS }

func (mfs *memFS) Open(name string) (File, error)        { return mfs.fs.Open(name) }
func (mfs *memFS) Create(name string) (File, error)      { return mfs.fs.Create(name) }
func (mfs *memFS) Stat(name string) (os.FileInfo, error) { return mfs.fs.Stat(name) }
func (mfs *memFS) Remove(name string) error              { return mfs.fs.Remove(name) }
func (mfs *memFS) CreateTemp(dir, pattern string) (File, error) {
	return mfs.fs.CreateTemp(dir, pattern)
}
func (mfs *memFS) MkdirAll(path string, perms os.FileMode) error { return mfs.fs.MkdirAll(path, perms) }
func (mfs *memFS) RemoveAll(path string) error                   { return mfs.fs.RemoveAll(path) }
func (mfs *memFS) ReadDir(name string) ([]os.DirEntry, error)    { return mfs.fs.ReadDir(name) }
func (mfs *memFS) MkdirTemp(dir, pattern string) (name string, err error) {
	return mfs.fs.MkdirTemp(dir, pattern)
}
func (mfs *memFS) TempDir() string { return mfs.fs.TempDir() }

func Test_Example(t *testing.T) {
	fs := &memFS{fs: memfs.New()}

	assert.NotNil(t, fs)
	assert.Equal(t, "/tmp", fs.TempDir())

	err := WriteContent(fs, "/test/test", []byte(`test data`))
	assert.Nil(t, err)

	data, err := GetContent(fs, "/test/test")
	assert.Nil(t, err)
	assert.Equal(t, `test data`, string(data))

}

// GetContent returns the contents for a File
func GetContent(fs FS, filename string) ([]byte, error) {
	file, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	filesize := fileInfo.Size()
	buffer := make([]byte, filesize)

	bytesRead, err := file.Read(buffer)
	if err != nil {
		return nil, err
	}

	if bytesRead != int(filesize) {
		return nil, errors.New("didn't read all of the File")
	}

	return buffer, nil
}

// WriteContent writes bytes to a File replacing the existing File or creating new
func WriteContent(fs FS, filename string, data []byte) error {
	dir := filepath.Dir(filename)
	if _, err := fs.Stat(dir); errors.Is(err, os.ErrNotExist) {
		_ = fs.MkdirAll(dir, 0700) // Create your File
	}

	f, err := fs.Create(filename)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	defer func() {
		_ = f.Close()
	}()

	_, err = w.Write(data)
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	return nil
}
