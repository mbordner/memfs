package memfs

import (
	"io/fs"
	"os"
)

type DirEntry struct {
	node *fsNode
}

func (de DirEntry) Name() string {
	return de.node.name
}

func (de DirEntry) IsDir() bool {
	return de.node.isDir()
}

func (de DirEntry) Type() os.FileMode {
	if de.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (de DirEntry) Info() (os.FileInfo, error) {
	return FileInfo{node: de.node}, nil
}
