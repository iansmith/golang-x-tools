// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package misc

import (
	"testing"

	. "github.com/iansmith/golang-x-tools/internal/lsp/regtest"
)

// This test passes (TestHoverOnError in definition_test.go) without
// the //line directive
func TestHoverFailure(t *testing.T) {
	const mod = `
-- go.mod --
module mod.com

go 1.12
-- a.y --
DWIM(main)

-- main.go --
//line a.y:1
package main

func main() {
	var err error
	err.Error()
}`
	WithOptions(SkipLogs()).Run(t, mod, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		content, _ := env.Hover("main.go", env.RegexpSearch("main.go", "Error"))
		// without the //line comment content would be non-nil
		if content != nil {
			t.Fatalf("expected nil hover content for Error")
		}
	})
}

// badPackageDup contains a duplicate definition of the 'a' const.
// this is from diagnostics_test.go,
const badPackageDup = `
-- go.mod --
module mod.com

go 1.12
-- a.go --
package consts

const a = 1
-- b.go --
package consts
//line gen.go:5
const a = 2
`

func TestFailingDiagnosticClearingOnEdit(t *testing.T) {
	Run(t, badPackageDup, func(t *testing.T, env *Env) {
		env.OpenFile("b.go")
		// no diagnostics for any files, but there should be
		env.Await(NoDiagnostics("a.go"), NoDiagnostics("b.go"))

		// Fix the error by editing the const name in b.go to `b`.
		env.RegexpReplace("b.go", "(a) = 2", "b")

		// The diagnostics that weren't sent above should now be cleared.
	})
}
