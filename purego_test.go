package zxinggo_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestPureGoNoCgo enforces that the library stays pure Go: no package may
// import "C" (cgo) or "unsafe". A CGO_ENABLED=0 build is the practical
// companion check; this test catches violations deterministically at test
// time without invoking the toolchain.
func TestPureGoNoCgo(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			switch strings.Trim(imp.Path.Value, `"`) {
			case "C":
				t.Errorf("%s: imports \"C\" (cgo)", path)
			case "unsafe":
				t.Errorf("%s: imports \"unsafe\"", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
