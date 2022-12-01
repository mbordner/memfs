package memfs

import (
	"github.com/stretchr/testify/assert"
	"io/fs"
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
