// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package checker_test

import (
	"fmt"
	"go/ast"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/iansmith/golang-x-tools/go/analysis"
	"github.com/iansmith/golang-x-tools/go/analysis/analysistest"
	"github.com/iansmith/golang-x-tools/go/analysis/internal/checker"
	"github.com/iansmith/golang-x-tools/go/analysis/passes/inspect"
	"github.com/iansmith/golang-x-tools/go/ast/inspector"
	"github.com/iansmith/golang-x-tools/internal/testenv"
)

var from, to string

func TestApplyFixes(t *testing.T) {
	testenv.NeedsGoPackages(t)

	from = "bar"
	to = "baz"

	files := map[string]string{
		"rename/test.go": `package rename

func Foo() {
	bar := 12
	_ = bar
}

// the end
`}
	want := `package rename

func Foo() {
	baz := 12
	_ = baz
}

// the end
`

	testdata, cleanup, err := analysistest.WriteFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testdata, "src/rename/test.go")
	checker.Fix = true
	checker.Run([]string{"file=" + path}, []*analysis.Analyzer{analyzer})

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	got := string(contents)
	if got != want {
		t.Errorf("contents of rewritten file\ngot: %s\nwant: %s", got, want)
	}

	defer cleanup()
}

var analyzer = &analysis.Analyzer{
	Name:     "rename",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{(*ast.Ident)(nil)}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		ident := n.(*ast.Ident)
		if ident.Name == from {
			msg := fmt.Sprintf("renaming %q to %q", from, to)
			pass.Report(analysis.Diagnostic{
				Pos:     ident.Pos(),
				End:     ident.End(),
				Message: msg,
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: msg,
					TextEdits: []analysis.TextEdit{{
						Pos:     ident.Pos(),
						End:     ident.End(),
						NewText: []byte(to),
					}},
				}},
			})
		}
	})

	return nil, nil
}

func TestRunDespiteErrors(t *testing.T) {
	testenv.NeedsGoPackages(t)

	files := map[string]string{
		"rderr/test.go": `package rderr

// Foo deliberately has a type error
func Foo(s string) int {
	return s + 1
}
`}

	testdata, cleanup, err := analysistest.WriteFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testdata, "src/rderr/test.go")

	// A no-op analyzer that should finish regardless of
	// parse or type errors in the code.
	noop := &analysis.Analyzer{
		Name:     "noop",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (interface{}, error) {
			return nil, nil
		},
		RunDespiteErrors: true,
	}

	for _, test := range []struct {
		name      string
		pattern   []string
		analyzers []*analysis.Analyzer
		code      int
	}{
		// parse/type errors
		{name: "skip-error", pattern: []string{"file=" + path}, analyzers: []*analysis.Analyzer{analyzer}, code: 1},
		{name: "despite-error", pattern: []string{"file=" + path}, analyzers: []*analysis.Analyzer{noop}, code: 0},
		// combination of parse/type errors and no errors
		{name: "despite-error-and-no-error", pattern: []string{"file=" + path, "sort"}, analyzers: []*analysis.Analyzer{analyzer, noop}, code: 1},
		// non-existing package error
		{name: "no-package", pattern: []string{"xyz"}, analyzers: []*analysis.Analyzer{analyzer}, code: 1},
		{name: "no-package-despite-error", pattern: []string{"abc"}, analyzers: []*analysis.Analyzer{noop}, code: 1},
		{name: "no-multi-package-despite-error", pattern: []string{"xyz", "abc"}, analyzers: []*analysis.Analyzer{noop}, code: 1},
		// combination of type/parsing and different errors
		{name: "different-errors", pattern: []string{"file=" + path, "xyz"}, analyzers: []*analysis.Analyzer{analyzer, noop}, code: 1},
		// non existing dir error
		{name: "no-match-dir", pattern: []string{"file=non/existing/dir"}, analyzers: []*analysis.Analyzer{analyzer, noop}, code: 1},
		// no errors
		{name: "no-errors", pattern: []string{"sort"}, analyzers: []*analysis.Analyzer{analyzer, noop}, code: 0},
	} {
		if got := checker.Run(test.pattern, test.analyzers); got != test.code {
			t.Errorf("got incorrect exit code %d for test %s; want %d", got, test.name, test.code)
		}
	}

	defer cleanup()
}
