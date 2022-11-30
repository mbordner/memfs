package memfs

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
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
	assert.Nil(t, err)
	assert.Equal(t, len(data), n)

	assert.Equal(t, data, string(readData))

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

	_, err = f.WriteAt([]byte(`deleted`), 0)
	assert.NotNil(t, err)

}
