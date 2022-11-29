package memfs_test

import (
	"fmt"
	"github.com/mbordner/memfs"
)

func ExampleNew() {
	fs := memfs.New()
	fmt.Println(fs.TempDir())

}
