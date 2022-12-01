package memfs

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func Test_Filename(t *testing.T) {
	f := new(fsNode)
	f.name = "test_file"

	tf := &File{node: f}

	assert.Equal(t, "test_file", tf.Name())
}

func Test_IsDir(t *testing.T) {
	f := new(fsNode)
	f.name = "test_file"

	d := new(fsNode)
	d.name = "test_dir"
	d.entries = make(map[string]*fsNode)

	assert.True(t, d.isDir())
	assert.False(t, f.isDir())

	assert.True(t, (&File{node: d}).isDir())
}

func Test_GetEntryNames(t *testing.T) {

	d := new(fsNode)
	d.entries = make(map[string]*fsNode)

	emptyNames := d.getEntryNames()
	assert.Len(t, emptyNames, 0)

	f := new(fsNode)
	emptyNames = f.getEntryNames()
	assert.Len(t, emptyNames, 0)

	entries := []*fsNode{
		&fsNode{name: "c"},
		&fsNode{name: "b"},
		&fsNode{name: "aa"},
		&fsNode{name: "a"},
	}

	for _, e := range entries {
		d.entries[e.name] = e
	}

	names := d.getEntryNames()
	assert.Len(t, names, 4)
	assert.Equal(t, "a", names[0])
	assert.Equal(t, "aa", names[1])
	assert.Equal(t, "b", names[2])
	assert.Equal(t, "c", names[3])
}

func Test_File_Operations(t *testing.T) {
	inMemFS := New()
	err := inMemFS.Mkdir("/", 0)

	err = inMemFS.Mkdir("/test", 0777)
	assert.Nil(t, err)
	err = inMemFS.Mkdir("/test", 0777)
	assert.NotNil(t, err)
	f, err := inMemFS.Create("/test/file1")
	assert.NotNil(t, f)
	assert.Nil(t, err)

	data := `test data`
	n, err := f.Write([]byte(data))
	assert.Nil(t, err)
	assert.Equal(t, len(data), n)

	readData := make([]byte, len(data), len(data))
	n, err = f.Read(readData)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, io.EOF))

	readData2 := make([]byte, len(data), len(data))
	n, err = f.ReadAt(readData2, 0)
	assert.Nil(t, err)
	assert.Equal(t, len(data), n)

	assert.Equal(t, data, string(readData2))

	err = f.Close()
	assert.Nil(t, err)

	_, err = f.Read(readData)
	assert.NotNil(t, err)

	_, err = f.ReadAt(readData, 0)
	assert.NotNil(t, err)

	err = f.Close()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrClosed))

	f, err = inMemFS.Open("/test/file1")
	assert.NotNil(t, f)
	assert.Nil(t, err)

	n, err = f.Write([]byte(`change`))
	assert.Equal(t, 0, n)
	assert.NotNil(t, err)

	err = f.Close()
	assert.Nil(t, err)

	f, err = inMemFS.OpenFile("/test/file1", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	assert.Nil(t, err)
	assert.NotNil(t, f)

	n, err = f.Write([]byte(`2`))
	assert.Equal(t, 1, n)
	assert.Nil(t, err)

	f2, err := inMemFS.Open("/test/file1")
	assert.Nil(t, err)
	assert.NotNil(t, f2)

	readData = make([]byte, len(data)+1, len(data)+1)
	n, err = f2.Read(readData)
	assert.Nil(t, err)
	assert.Equal(t, len(readData), n)

	assert.Equal(t, `test data2`, string(readData))

	n, err = f.WriteAt([]byte(`3`), 9)
	assert.NotNil(t, err)
	err = f.Close()
	assert.Nil(t, err)
	f, err = inMemFS.OpenFile("/test/file1", os.O_RDWR|os.O_CREATE, 0777)
	assert.Nil(t, err)
	assert.NotNil(t, f)
	n, err = f.WriteAt([]byte(`3`), 9)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)

	n, err = f2.Read(readData)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, io.EOF))
	assert.Equal(t, 0, n)

	p, err := f2.Seek(0, io.SeekStart)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), p)

	n, err = f2.Read(readData)
	assert.Nil(t, err)
	assert.Equal(t, len(readData), n)

	assert.Equal(t, `test data3`, string(readData))

	p, err = f.Seek(-2, io.SeekEnd)
	assert.Nil(t, err)
	assert.Equal(t, int64(p), p)

	n, err = f.Write([]byte(`A4`))
	assert.Nil(t, err)
	assert.Equal(t, 2, n)

	p, err = f2.Seek(0, io.SeekStart)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), p)

	n, err = f2.Read(readData)
	assert.Nil(t, err)
	assert.Equal(t, len(readData), n)

	assert.Equal(t, `test datA4`, string(readData))

	p, err = f.Seek(-1, io.SeekCurrent)
	assert.Nil(t, err)
	assert.Equal(t, int64(9), p)

	n, err = f.Write([]byte(`42`))
	assert.Nil(t, err)
	assert.Equal(t, 2, n)

	n, err = f.WriteAt([]byte(`42`), -4)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	p, err = f2.Seek(0, io.SeekStart)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), p)

	readData3 := make([]byte, len(readData)+1, len(readData)+1)

	n, err = f2.Read(readData3)
	assert.Nil(t, err)
	assert.Equal(t, len(readData3), n)

	assert.Equal(t, `test datA42`, string(readData3))

	p, err = f2.Seek(-1, io.SeekStart)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	n, err = f2.ReadAt([]byte(` `), -1)
	assert.NotNil(t, err)

	err = f.Close()
	assert.Nil(t, err)

	p, err = f.Seek(0, io.SeekStart)
	assert.NotNil(t, err)

	n, err = f.Read(readData)
	assert.NotNil(t, err)

	n, err = f.ReadAt(readData, 0)
	assert.NotNil(t, err)

	s, err := f.Stat()
	assert.NotNil(t, s)
	assert.Nil(t, err)

	_, err = f.Write([]byte(`deleted`))
	assert.NotNil(t, err)

	_, err = f.WriteAt([]byte(`deleted`), 0)
	assert.NotNil(t, err)

	err = inMemFS.Remove("/test/file1")
	assert.Nil(t, err)

	p, err = f2.Seek(0, io.SeekStart)
	assert.NotNil(t, err)

	n, err = f2.Read(readData)
	assert.NotNil(t, err)

	n, err = f2.ReadAt(readData, 0)
	assert.NotNil(t, err)

	_, err = f2.Stat()
	assert.NotNil(t, err)

	_, err = f2.Write([]byte(`deleted`))
	assert.NotNil(t, err)

	_, err = f2.WriteAt([]byte(`deleted`), 0)
	assert.NotNil(t, err)

	f3, err := inMemFS.OpenFile("/test/file1", os.O_WRONLY|os.O_CREATE, 0777)
	assert.Nil(t, err)
	assert.NotNil(t, f3)

	n, err = f3.Read(readData)
	assert.NotNil(t, err)

	n, err = f3.ReadAt(readData, 0)
	assert.NotNil(t, err)

	f4, err := inMemFS.OpenFile("/test/file1", os.O_RDONLY|os.O_CREATE, 0777)
	assert.Nil(t, err)
	assert.NotNil(t, f4)

	n, err = f4.WriteAt(readData, 0)
	assert.NotNil(t, err)

	relPath := "file1"
	wd, err := os.Getwd()
	assert.Nil(t, err)
	assert.NotEmpty(t, wd)
	f5, err := inMemFS.Create(relPath)
	assert.Nil(t, err)
	assert.NotNil(t, f5)

	absPath := filepath.Join(wd, relPath)
	fi, err := inMemFS.Stat(absPath)
	assert.Nil(t, err)
	assert.NotNil(t, fi)

	assert.Equal(t, relPath, fi.Name())
	assert.False(t, fi.IsDir())

	f6, err := inMemFS.CreateTemp("", "blah*blah*blah*")
	assert.NotNil(t, f6)
	assert.Nil(t, err)

	s, err = f6.Stat()
	assert.IsType(t, "", s.Name())
	assert.False(t, s.IsDir())
	assert.Greater(t, int(s.Mode()), 0)
	assert.NotNil(t, s.ModTime())
}

func Test_ReadDirFuncs(t *testing.T) {

	inMemFS := New()
	tmpDirName, err := inMemFS.MkdirTemp("", "test*")
	assert.Nil(t, err)
	assert.Contains(t, tmpDirName, "test")

	err = inMemFS.Remove(tmpDirName)
	assert.Nil(t, err)

	tmpDirName, err = inMemFS.MkdirTemp("", "test*")
	assert.Nil(t, err)
	assert.Contains(t, tmpDirName, "test")

	files := make([]*File, 10, 10)
	for i := range files {
		f, err := inMemFS.CreateTemp(tmpDirName, "testfile")
		assert.Nil(t, err)
		assert.NotNil(t, f)
		files[i] = f
	}

	tfi, err := files[0].Readdir(-1)
	assert.NotNil(t, err)
	assert.Nil(t, tfi)

	tde, err := files[0].ReadDir(-1)
	assert.NotNil(t, err)
	assert.Nil(t, tde)

	tn, err := files[0].Readdirnames(-1)
	assert.NotNil(t, err)
	assert.Nil(t, tn)

	dir, err := inMemFS.Open(tmpDirName)
	assert.Nil(t, err)
	assert.NotNil(t, dir)

	s, err := dir.Stat()
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, 0, int(s.Size()))
	assert.Nil(t, s.Sys())

	names, err := dir.Readdirnames(-1)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(names))

	names, err = dir.Readdirnames(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(names))

	names, err = dir.Readdirnames(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(names))

	entries, err := dir.ReadDir(-1)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(entries))

	assert.IsType(t, "", entries[0].Name())
	assert.False(t, entries[0].IsDir())
	assert.Equal(t, 0, int(entries[0].Type()))
	di, err := entries[0].Info()
	assert.NotNil(t, di)
	assert.Nil(t, err)

	entries, err = dir.ReadDir(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(entries))

	entries, err = dir.ReadDir(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(entries))

	infos, err := dir.Readdir(-1)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(infos))

	infos, err = dir.Readdir(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(infos))

	infos, err = dir.Readdir(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(infos))

	entries, err = inMemFS.ReadDir(tmpDirName)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(entries))

	entries, err = inMemFS.ReadDir(string([]byte{0x52, 0xE4, 0x76}))
	assert.Nil(t, entries)
	assert.NotNil(t, err)

	err = dir.Close()
	assert.Nil(t, err)

	names, err = dir.Readdirnames(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrClosed))

	infos, err = dir.Readdir(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrClosed))

	entries, err = dir.ReadDir(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrClosed))

	err = inMemFS.Remove(tmpDirName)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	err = inMemFS.RemoveAll(tmpDirName)
	assert.Nil(t, err)

	names, err = dir.Readdirnames(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	infos, err = dir.Readdir(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	entries, err = dir.ReadDir(5)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	err = inMemFS.RemoveAll(string([]byte{0x52, 0xE4, 0x76}))
	assert.NotNil(t, err)

	n, err := inMemFS.MkdirTemp(string([]byte{0x52, 0xE4, 0x76}), string([]byte{0x52, 0xE4, 0x76}))
	assert.NotNil(t, err)
	assert.Equal(t, "", n)

	f, err := inMemFS.CreateTemp("/blah", "blah")
	assert.NotNil(t, err)
	assert.Nil(t, f)
	assert.True(t, errors.Is(err, os.ErrNotExist))

}
