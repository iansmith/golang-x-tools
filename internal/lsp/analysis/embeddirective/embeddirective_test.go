// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package embeddirective

import (
	"testing"

	"github.com/iansmith/golang-x-tools/go/analysis/analysistest"
	"github.com/iansmith/golang-x-tools/internal/typeparams"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	tests := []string{"a"}
	if typeparams.Enabled {
		tests = append(tests)
	}

	analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, tests...)
}
