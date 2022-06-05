package nodisk

import (
	"github.com/iansmith/golang-x-tools/internal/lsp/foo"
)

func _() {
	foo.Foo() //@complete("F", Foo, IntFoo, StructFoo)
}
