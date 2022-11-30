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

	err = f.Close()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, os.ErrClosed))

	f, err = inMemFS.Open("/test/file1")
	assert.NotNil(t, f)
	assert.Nil(t, err)

	n, err = f.Write([]byte(`change`))
	assert.Equal(t, 0, n)
	assert.NotNil(t, err)

}
