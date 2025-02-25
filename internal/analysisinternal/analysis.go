// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package analysisinternal exposes internal-only fields from go/analysis.
package analysisinternal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/iansmith/golang-x-tools/go/ast/astutil"
	"github.com/iansmith/golang-x-tools/internal/lsp/fuzzy"
)

// Flag to gate diagnostics for fuzz tests in 1.18.
var DiagnoseFuzzTests bool = false

var (
	GetTypeErrors func(p interface{}) []types.Error
	SetTypeErrors func(p interface{}, errors []types.Error)
)

func TypeErrorEndPos(fset *token.FileSet, src []byte, start token.Pos) token.Pos {
	// Get the end position for the type error.
	offset, end := fset.PositionFor(start, false).Offset, start
	if offset >= len(src) {
		return end
	}
	if width := bytes.IndexAny(src[offset:], " \n,():;[]+-*"); width > 0 {
		end = start + token.Pos(width)
	}
	return end
}

func ZeroValue(fset *token.FileSet, f *ast.File, pkg *types.Package, typ types.Type) ast.Expr {
	under := typ
	if n, ok := typ.(*types.Named); ok {
		under = n.Underlying()
	}
	switch u := under.(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsNumeric != 0:
			return &ast.BasicLit{Kind: token.INT, Value: "0"}
		case u.Info()&types.IsBoolean != 0:
			return &ast.Ident{Name: "false"}
		case u.Info()&types.IsString != 0:
			return &ast.BasicLit{Kind: token.STRING, Value: `""`}
		default:
			panic("unknown basic type")
		}
	case *types.Chan, *types.Interface, *types.Map, *types.Pointer, *types.Signature, *types.Slice, *types.Array:
		return ast.NewIdent("nil")
	case *types.Struct:
		texpr := TypeExpr(fset, f, pkg, typ) // typ because we want the name here.
		if texpr == nil {
			return nil
		}
		return &ast.CompositeLit{
			Type: texpr,
		}
	}
	return nil
}

// IsZeroValue checks whether the given expression is a 'zero value' (as determined by output of
// analysisinternal.ZeroValue)
func IsZeroValue(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value == "0" || e.Value == `""`
	case *ast.Ident:
		return e.Name == "nil" || e.Name == "false"
	default:
		return false
	}
}

func TypeExpr(fset *token.FileSet, f *ast.File, pkg *types.Package, typ types.Type) ast.Expr {
	switch t := typ.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.UnsafePointer:
			return &ast.SelectorExpr{X: ast.NewIdent("unsafe"), Sel: ast.NewIdent("Pointer")}
		default:
			return ast.NewIdent(t.Name())
		}
	case *types.Pointer:
		x := TypeExpr(fset, f, pkg, t.Elem())
		if x == nil {
			return nil
		}
		return &ast.UnaryExpr{
			Op: token.MUL,
			X:  x,
		}
	case *types.Array:
		elt := TypeExpr(fset, f, pkg, t.Elem())
		if elt == nil {
			return nil
		}
		return &ast.ArrayType{
			Len: &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%d", t.Len()),
			},
			Elt: elt,
		}
	case *types.Slice:
		elt := TypeExpr(fset, f, pkg, t.Elem())
		if elt == nil {
			return nil
		}
		return &ast.ArrayType{
			Elt: elt,
		}
	case *types.Map:
		key := TypeExpr(fset, f, pkg, t.Key())
		value := TypeExpr(fset, f, pkg, t.Elem())
		if key == nil || value == nil {
			return nil
		}
		return &ast.MapType{
			Key:   key,
			Value: value,
		}
	case *types.Chan:
		dir := ast.ChanDir(t.Dir())
		if t.Dir() == types.SendRecv {
			dir = ast.SEND | ast.RECV
		}
		value := TypeExpr(fset, f, pkg, t.Elem())
		if value == nil {
			return nil
		}
		return &ast.ChanType{
			Dir:   dir,
			Value: value,
		}
	case *types.Signature:
		var params []*ast.Field
		for i := 0; i < t.Params().Len(); i++ {
			p := TypeExpr(fset, f, pkg, t.Params().At(i).Type())
			if p == nil {
				return nil
			}
			params = append(params, &ast.Field{
				Type: p,
				Names: []*ast.Ident{
					{
						Name: t.Params().At(i).Name(),
					},
				},
			})
		}
		var returns []*ast.Field
		for i := 0; i < t.Results().Len(); i++ {
			r := TypeExpr(fset, f, pkg, t.Results().At(i).Type())
			if r == nil {
				return nil
			}
			returns = append(returns, &ast.Field{
				Type: r,
			})
		}
		return &ast.FuncType{
			Params: &ast.FieldList{
				List: params,
			},
			Results: &ast.FieldList{
				List: returns,
			},
		}
	case *types.Named:
		if t.Obj().Pkg() == nil {
			return ast.NewIdent(t.Obj().Name())
		}
		if t.Obj().Pkg() == pkg {
			return ast.NewIdent(t.Obj().Name())
		}
		pkgName := t.Obj().Pkg().Name()
		// If the file already imports the package under another name, use that.
		for _, group := range astutil.Imports(fset, f) {
			for _, cand := range group {
				if strings.Trim(cand.Path.Value, `"`) == t.Obj().Pkg().Path() {
					if cand.Name != nil && cand.Name.Name != "" {
						pkgName = cand.Name.Name
					}
				}
			}
		}
		if pkgName == "." {
			return ast.NewIdent(t.Obj().Name())
		}
		return &ast.SelectorExpr{
			X:   ast.NewIdent(pkgName),
			Sel: ast.NewIdent(t.Obj().Name()),
		}
	case *types.Struct:
		return ast.NewIdent(t.String())
	case *types.Interface:
		return ast.NewIdent(t.String())
	default:
		return nil
	}
}

type TypeErrorPass string

const (
	NoNewVars      TypeErrorPass = "nonewvars"
	NoResultValues TypeErrorPass = "noresultvalues"
	UndeclaredName TypeErrorPass = "undeclaredname"
)

// StmtToInsertVarBefore returns the ast.Stmt before which we can safely insert a new variable.
// Some examples:
//
// Basic Example:
// z := 1
// y := z + x
// If x is undeclared, then this function would return `y := z + x`, so that we
// can insert `x := ` on the line before `y := z + x`.
//
// If stmt example:
// if z == 1 {
// } else if z == y {}
// If y is undeclared, then this function would return `if z == 1 {`, because we cannot
// insert a statement between an if and an else if statement. As a result, we need to find
// the top of the if chain to insert `y := ` before.
func StmtToInsertVarBefore(path []ast.Node) ast.Stmt {
	enclosingIndex := -1
	for i, p := range path {
		if _, ok := p.(ast.Stmt); ok {
			enclosingIndex = i
			break
		}
	}
	if enclosingIndex == -1 {
		return nil
	}
	enclosingStmt := path[enclosingIndex]
	switch enclosingStmt.(type) {
	case *ast.IfStmt:
		// The enclosingStmt is inside of the if declaration,
		// We need to check if we are in an else-if stmt and
		// get the base if statement.
		return baseIfStmt(path, enclosingIndex)
	case *ast.CaseClause:
		// Get the enclosing switch stmt if the enclosingStmt is
		// inside of the case statement.
		for i := enclosingIndex + 1; i < len(path); i++ {
			if node, ok := path[i].(*ast.SwitchStmt); ok {
				return node
			} else if node, ok := path[i].(*ast.TypeSwitchStmt); ok {
				return node
			}
		}
	}
	if len(path) <= enclosingIndex+1 {
		return enclosingStmt.(ast.Stmt)
	}
	// Check if the enclosing statement is inside another node.
	switch expr := path[enclosingIndex+1].(type) {
	case *ast.IfStmt:
		// Get the base if statement.
		return baseIfStmt(path, enclosingIndex+1)
	case *ast.ForStmt:
		if expr.Init == enclosingStmt || expr.Post == enclosingStmt {
			return expr
		}
	}
	return enclosingStmt.(ast.Stmt)
}

// baseIfStmt walks up the if/else-if chain until we get to
// the top of the current if chain.
func baseIfStmt(path []ast.Node, index int) ast.Stmt {
	stmt := path[index]
	for i := index + 1; i < len(path); i++ {
		if node, ok := path[i].(*ast.IfStmt); ok && node.Else == stmt {
			stmt = node
			continue
		}
		break
	}
	return stmt.(ast.Stmt)
}

// WalkASTWithParent walks the AST rooted at n. The semantics are
// similar to ast.Inspect except it does not call f(nil).
func WalkASTWithParent(n ast.Node, f func(n ast.Node, parent ast.Node) bool) {
	var ancestors []ast.Node
	ast.Inspect(n, func(n ast.Node) (recurse bool) {
		if n == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return false
		}

		var parent ast.Node
		if len(ancestors) > 0 {
			parent = ancestors[len(ancestors)-1]
		}
		ancestors = append(ancestors, n)
		return f(n, parent)
	})
}

// FindMatchingIdents finds all identifiers in 'node' that match any of the given types.
// 'pos' represents the position at which the identifiers may be inserted. 'pos' must be within
// the scope of each of identifier we select. Otherwise, we will insert a variable at 'pos' that
// is unrecognized.
func FindMatchingIdents(typs []types.Type, node ast.Node, pos token.Pos, info *types.Info, pkg *types.Package) map[types.Type][]*ast.Ident {
	matches := map[types.Type][]*ast.Ident{}
	// Initialize matches to contain the variable types we are searching for.
	for _, typ := range typs {
		if typ == nil {
			continue
		}
		matches[typ] = []*ast.Ident{}
	}
	seen := map[types.Object]struct{}{}
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// Prevent circular definitions. If 'pos' is within an assignment statement, do not
		// allow any identifiers in that assignment statement to be selected. Otherwise,
		// we could do the following, where 'x' satisfies the type of 'f0':
		//
		// x := fakeStruct{f0: x}
		//
		assignment, ok := n.(*ast.AssignStmt)
		if ok && pos > assignment.Pos() && pos <= assignment.End() {
			return false
		}
		if n.End() > pos {
			return n.Pos() <= pos
		}
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name == "_" {
			return true
		}
		obj := info.Defs[ident]
		if obj == nil || obj.Type() == nil {
			return true
		}
		if _, ok := obj.(*types.TypeName); ok {
			return true
		}
		// Prevent duplicates in matches' values.
		if _, ok = seen[obj]; ok {
			return true
		}
		seen[obj] = struct{}{}
		// Find the scope for the given position. Then, check whether the object
		// exists within the scope.
		innerScope := pkg.Scope().Innermost(pos)
		if innerScope == nil {
			return true
		}
		_, foundObj := innerScope.LookupParent(ident.Name, pos)
		if foundObj != obj {
			return true
		}
		// The object must match one of the types that we are searching for.
		if idents, ok := matches[obj.Type()]; ok {
			matches[obj.Type()] = append(idents, ast.NewIdent(ident.Name))
		}
		// If the object type does not exactly match any of the target types, greedily
		// find the first target type that the object type can satisfy.
		for typ := range matches {
			if obj.Type() == typ {
				continue
			}
			if equivalentTypes(obj.Type(), typ) {
				matches[typ] = append(matches[typ], ast.NewIdent(ident.Name))
			}
		}
		return true
	})
	return matches
}

func equivalentTypes(want, got types.Type) bool {
	if want == got || types.Identical(want, got) {
		return true
	}
	// Code segment to help check for untyped equality from (golang/go#32146).
	if rhs, ok := want.(*types.Basic); ok && rhs.Info()&types.IsUntyped > 0 {
		if lhs, ok := got.Underlying().(*types.Basic); ok {
			return rhs.Info()&types.IsConstType == lhs.Info()&types.IsConstType
		}
	}
	return types.AssignableTo(want, got)
}

// FindBestMatch employs fuzzy matching to evaluate the similarity of each given identifier to the
// given pattern. We return the identifier whose name is most similar to the pattern.
func FindBestMatch(pattern string, idents []*ast.Ident) ast.Expr {
	fuzz := fuzzy.NewMatcher(pattern)
	var bestFuzz ast.Expr
	highScore := float32(0) // minimum score is 0 (no match)
	for _, ident := range idents {
		// TODO: Improve scoring algorithm.
		score := fuzz.Score(ident.Name)
		if score > highScore {
			highScore = score
			bestFuzz = ident
		} else if score == 0 {
			// Order matters in the fuzzy matching algorithm. If we find no match
			// when matching the target to the identifier, try matching the identifier
			// to the target.
			revFuzz := fuzzy.NewMatcher(ident.Name)
			revScore := revFuzz.Score(pattern)
			if revScore > highScore {
				highScore = revScore
				bestFuzz = ident
			}
		}
	}
	return bestFuzz
}
