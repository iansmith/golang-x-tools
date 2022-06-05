// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hooks adds all the standard gopls implementations.
// This can be used in tests without needing to use the gopls main, and is
// also the place to edit for custom builds of gopls.
package hooks // import "github.com/iansmith/golang-x-tools/gopls/internal/hooks"

import (
	"context"

	"github.com/iansmith/golang-x-tools/gopls/internal/vulncheck"
	"github.com/iansmith/golang-x-tools/internal/lsp/source"
	"mvdan.cc/gofumpt/format"
	"mvdan.cc/xurls/v2"
)

func Options(options *source.Options) {
	options.LicensesText = licensesText
	if options.GoDiff {
		options.ComputeEdits = ComputeEdits
	}
	options.URLRegexp = xurls.Relaxed()
	options.GofumptFormat = func(ctx context.Context, langVersion, modulePath string, src []byte) ([]byte, error) {
		return format.Source(src, format.Options{
			LangVersion: langVersion,
			ModulePath:  modulePath,
		})
	}
	updateAnalyzers(options)

	options.Govulncheck = vulncheck.Govulncheck
}
