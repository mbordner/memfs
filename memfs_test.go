package memfs

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"os"
	"testing"
)

func Test_MkdirAll(t *testing.T) {
	mfs := New()
	assert.NotNil(t, mfs)

	err := mfs.MkdirAll("/test/test/test", fs.ModePerm)
	assert.Nil(t, err)

	err = mfs.MkdirAll("/test/test/test", fs.ModePerm)
	assert.Nil(t, err)
}

func Test_Try_Path_With_File_Part(t *testing.T) {

	mfs := New()

	assert.Nil(t, mfs.Mkdir("/testDir", 0777))
	assert.Nil(t, mfs.Mkdir("/testDir/testDir2", 0777))

	dir, err := mfs.Open("/testDir")
	assert.Nil(t, err)
	assert.NotNil(t, dir)

	f, err := mfs.Create("/testDir/file1")
	assert.Nil(t, err)
	assert.NotNil(t, f)

	entries, err := dir.ReadDir(-1)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(entries))

	for _, e := range entries {
		switch e.Name() {
		case "file1":
			assert.False(t, e.IsDir())
			assert.Equal(t, 0, int(e.Type()))
		case "testDir2":
			assert.True(t, e.IsDir())
			assert.Equal(t, fs.ModeDir, e.Type())
		}
	}

	err = mfs.MkdirAll(string([]byte{0x52, 0xE4, 0x76}), 0)
	assert.NotNil(t, err)

	err = mfs.MkdirAll("", 0)
	assert.NotNil(t, err)

	err = mfs.MkdirAll("/testDir/file1/testDir3", 0777)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	f2, err := mfs.Create("/testDir/testDir4/testDir5/file2")
	assert.NotNil(t, err)
	assert.Nil(t, f2)

	f3, err := mfs.Create("/testDir/file1/testDir4/testDir5/file2")
	assert.NotNil(t, err)
	assert.Nil(t, f3)

	err = mfs.Mkdir("/testDir/testDir4/testDir5/testDir6", 0)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))

}

func Test_Missing_Dir_Paths(t *testing.T) {
	mfs := New()

	err := mfs.Remove("/test/test1")
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))

	err = mfs.RemoveAll("/test/test1")
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))

	entries, err := mfs.ReadDir("/test/test1")
	assert.NotNil(t, err)
	assert.Nil(t, entries)
	assert.True(t, errors.Is(err, os.ErrNotExist))

	name, err := mfs.MkdirTemp("/test/test1", "kljsdf")
	assert.NotNil(t, err)
	assert.Equal(t, "", name)

}

func Test_Non_UTF8_File_Path(t *testing.T) {

	mfs := New()

	invalidUTF8Path := string([]byte{0x52, 0xE4, 0x76})

	f, err := mfs.Create(invalidUTF8Path)
	assert.NotNil(t, err)
	assert.Nil(t, f)

	_, err = mfs.Stat(invalidUTF8Path)
	assert.NotNil(t, err)

	err = mfs.Remove(invalidUTF8Path)
	assert.NotNil(t, err)

	err = mfs.Mkdir(invalidUTF8Path, 0)
	assert.NotNil(t, err)

	f, err = mfs.CreateTemp(invalidUTF8Path, "kjsdf")
	assert.NotNil(t, err)
	assert.Nil(t, f)
}

func Test_Open_Mode_Issues(t *testing.T) {

	mfs := New()

	assert.Nil(t, mfs.Mkdir("/testDir", 0777))
	assert.Nil(t, mfs.Mkdir("/testDir/testDir2", 0777))

	dir, err := mfs.Open("/testDir")
	assert.Nil(t, err)
	assert.NotNil(t, dir)

	f, err := mfs.Create("/testDir/file1")
	assert.Nil(t, err)
	assert.NotNil(t, f)

	f2, err := mfs.OpenFile("/testDir/file1", os.O_CREATE|os.O_RDWR|os.O_EXCL, 0777)
	assert.NotNil(t, err)
	assert.Nil(t, f2)
	assert.True(t, errors.Is(err, os.ErrExist))

	f3, err := mfs.OpenFile("/testDir/file1", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0777)
	assert.Nil(t, err)
	assert.NotNil(t, f3)
	s, err := f3.Stat()
	assert.Nil(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, int64(0), s.Size())

	f4, err := mfs.OpenFile("/testDir/file3", os.O_WRONLY, 0777)
	assert.NotNil(t, err)
	assert.Nil(t, f4)
	assert.True(t, errors.Is(err, os.ErrInvalid))

	f5, err := mfs.OpenFile("/testDir/file3", os.O_RDONLY, 0777)
	assert.NotNil(t, err)
	assert.Nil(t, f5)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}
