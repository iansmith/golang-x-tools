// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printf_test

import (
	"testing"

	"github.com/iansmith/golang-x-tools/go/analysis/analysistest"
	"github.com/iansmith/golang-x-tools/go/analysis/passes/printf"
	"github.com/iansmith/golang-x-tools/internal/typeparams"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	printf.Analyzer.Flags.Set("funcs", "Warn,Warnf")

	tests := []string{"a", "b", "nofmt"}
	if typeparams.Enabled {
		tests = append(tests, "typeparams")
	}
	analysistest.Run(t, testdata, printf.Analyzer, tests...)
}
