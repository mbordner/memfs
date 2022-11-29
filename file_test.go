package memfs

import (
	"github.com/stretchr/testify/assert"
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
