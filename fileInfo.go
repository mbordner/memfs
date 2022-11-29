package memfs

import (
	"os"
	"time"
)

type FileInfo struct {
	node *fsNode
}

func (fi FileInfo) Name() string {
	return fi.node.name
}

func (fi FileInfo) Size() int64 {
	if !fi.node.unlinked {
		fi.node.mutex.Lock()
		defer fi.node.mutex.Unlock()
		if !fi.node.isDir() {
			return int64(len(fi.node.content))
		}
	}
	return 0
}

func (fi FileInfo) Mode() os.FileMode {
	return fi.node.perm
}

func (fi FileInfo) ModTime() time.Time {
	return fi.node.modified
}

func (fi FileInfo) IsDir() bool {
	return fi.node.isDir()
}

func (fi FileInfo) Sys() any {
	return nil
}
