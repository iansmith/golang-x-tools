// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package misc

import (
	"testing"

	. "github.com/iansmith/golang-x-tools/internal/lsp/regtest"

	"github.com/iansmith/golang-x-tools/internal/lsp/protocol"
	"github.com/iansmith/golang-x-tools/internal/testenv"
)

const basicProxy = `
-- golang.org/x/hello@v1.2.3/go.mod --
module golang.org/x/hello

go 1.14
-- golang.org/x/hello@v1.2.3/hi/hi.go --
package hi

var Goodbye error
`

func TestInconsistentVendoring(t *testing.T) {
	testenv.NeedsGo1Point(t, 14)

	// TODO(golang/go#49646): delete this comment once this test is stable.
	//
	// In golang/go#49646, this test is reported as flaky on Windows. We believe
	// this is due to file contention from go mod vendor that should be resolved.
	// If this test proves to still be flaky, skip it.
	//
	// if runtime.GOOS == "windows" {
	// 	t.Skipf("skipping test due to flakiness on Windows: https://golang.org/issue/49646")
	// }

	const pkgThatUsesVendoring = `
-- go.mod --
module mod.com

go 1.14

require golang.org/x/hello v1.2.3
-- go.sum --
golang.org/x/hello v1.2.3 h1:EcMp5gSkIhaTkPXp8/3+VH+IFqTpk3ZbpOhqk0Ncmho=
golang.org/x/hello v1.2.3/go.mod h1:WW7ER2MRNXWA6c8/4bDIek4Hc/+DofTrMaQQitGXcco=
-- vendor/modules.txt --
-- a/a1.go --
package a

import "golang.org/x/hello/hi"

func _() {
	_ = hi.Goodbye
	var q int // hardcode a diagnostic
}
`
	WithOptions(
		Modes(Singleton),
		ProxyFiles(basicProxy),
	).Run(t, pkgThatUsesVendoring, func(t *testing.T, env *Env) {
		env.OpenFile("a/a1.go")
		d := &protocol.PublishDiagnosticsParams{}
		env.Await(
			OnceMet(
				env.DiagnosticAtRegexpWithMessage("go.mod", "module mod.com", "Inconsistent vendoring"),
				ReadDiagnostics("go.mod", d),
			),
		)
		env.ApplyQuickFixes("go.mod", d.Diagnostics)

		env.Await(
			env.DiagnosticAtRegexpWithMessage("a/a1.go", `q int`, "not used"),
		)
	})
}
