// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gopls_test

import (
	"os"
	"testing"

	"github.com/iansmith/golang-x-tools/gopls/internal/hooks"
	"github.com/iansmith/golang-x-tools/internal/lsp/bug"
	cmdtest "github.com/iansmith/golang-x-tools/internal/lsp/cmd/test"
	"github.com/iansmith/golang-x-tools/internal/lsp/source"
	"github.com/iansmith/golang-x-tools/internal/lsp/tests"
	"github.com/iansmith/golang-x-tools/internal/testenv"
)

func TestMain(m *testing.M) {
	bug.PanicOnBugs = true
	testenv.ExitIfSmallMachine()
	os.Exit(m.Run())
}

func TestCommandLine(t *testing.T) {
	cmdtest.TestCommandLine(t, "../../internal/lsp/testdata", commandLineOptions)
}

func commandLineOptions(options *source.Options) {
	options.Staticcheck = true
	options.GoDiff = false
	tests.DefaultOptions(options)
	hooks.Options(options)
}
