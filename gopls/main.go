// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gopls (pronounced “go please”) is an LSP server for Go.
// The Language Server Protocol allows any text editor
// to be extended with IDE-like features;
// see https://langserver.org/ for details.
//
// See https://github.com/golang/tools/blob/master/gopls/README.md
// for the most up-to-date documentation.
package main // import "github.com/iansmith/golang-x-tools/gopls"

import (
	"context"
	"github.com/iansmith/golang-x-tools/internal/analysisinternal"
	"os"

	"github.com/iansmith/golang-x-tools/gopls/internal/hooks"
	"github.com/iansmith/golang-x-tools/internal/lsp/cmd"
	"github.com/iansmith/golang-x-tools/internal/tool"
)

func main() {
	// In 1.18, diagnostics for Fuzz tests must not be used by cmd/vet.
	// So the code for Fuzz tests diagnostics is guarded behind flag analysisinternal.DiagnoseFuzzTests
	// Turn on analysisinternal.DiagnoseFuzzTests for gopls
	analysisinternal.DiagnoseFuzzTests = true
	ctx := context.Background()
	tool.Main(ctx, cmd.New("gopls", "", nil, hooks.Options), os.Args[1:])
}
