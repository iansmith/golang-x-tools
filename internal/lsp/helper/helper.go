// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Invoke with //go:generate helper/helper -t Server -d protocol/tsserver.go -u lsp -o server_gen.go
// invoke in internal/lsp
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
)

var (
	typ = flag.String("t", "Server", "generate code for this type")
	def = flag.String("d", "", "the file the type is defined in") // this relies on punning
	use = flag.String("u", "", "look for uses in this package")
	out = flag.String("o", "", "where to write the generated file")
)

func main() {
	log.SetFlags(log.Lshortfile)
	flag.Parse()
	if *typ == "" || *def == "" || *use == "" || *out == "" {
		flag.PrintDefaults()
		return
	}
	// read the type definition and see what methods we're looking for
	doTypes()

	// parse the package and see which methods are defined
	doUses()

	output()
}

// replace "\\\n" with nothing before using
var tmpl = `// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

// code generated by helper. DO NOT EDIT.

import (
	"context"

	"github.com/iansmith/golang-x-tools/internal/lsp/protocol"
)

{{range $key, $v := .Stuff}}
func (s *{{$.Type}}) {{$v.Name}}({{.Param}}) {{.Result}} {
	{{if ne .Found ""}} return s.{{.Internal}}({{.Invoke}})\
	{{else}}return {{if lt 1 (len .Results)}}nil, {{end}}notImplemented("{{.Name}}"){{end}}
}
{{end}}
`

func output() {
	// put in empty param names as needed
	for _, t := range types {
		if t.paramnames == nil {
			t.paramnames = make([]string, len(t.paramtypes))
		}
		for i, p := range t.paramtypes {
			cm := ""
			if i > 0 {
				cm = ", "
			}
			t.Param += fmt.Sprintf("%s%s %s", cm, t.paramnames[i], p)
			this := t.paramnames[i]
			if this == "_" {
				this = "nil"
			}
			t.Invoke += fmt.Sprintf("%s%s", cm, this)
		}
		if len(t.Results) > 1 {
			t.Result = "("
		}
		for i, r := range t.Results {
			cm := ""
			if i > 0 {
				cm = ", "
			}
			t.Result += fmt.Sprintf("%s%s", cm, r)
		}
		if len(t.Results) > 1 {
			t.Result += ")"
		}
	}

	fd, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	t, err := template.New("foo").Parse(tmpl)
	if err != nil {
		log.Fatal(err)
	}
	type par struct {
		Type  string
		Stuff []*Function
	}
	p := par{*typ, types}
	if false { // debugging the template
		t.Execute(os.Stderr, &p)
	}
	buf := bytes.NewBuffer(nil)
	err = t.Execute(buf, &p)
	if err != nil {
		log.Fatal(err)
	}
	ans, err := format.Source(bytes.Replace(buf.Bytes(), []byte("\\\n"), []byte{}, -1))
	if err != nil {
		log.Fatal(err)
	}
	fd.Write(ans)
}

func doUses() {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, *use, nil, 0)
	if err != nil {
		log.Fatalf("%q:%v", *use, err)
	}
	pkg := pkgs["lsp"] // CHECK
	files := pkg.Files
	for fname, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			nm := fd.Name.String()
			if ast.IsExported(nm) {
				// we're looking for things like didChange
				continue
			}
			if fx, ok := byname[nm]; ok {
				if fx.Found != "" {
					log.Fatalf("found %s in %s and %s", fx.Internal, fx.Found, fname)
				}
				fx.Found = fname
				// and the Paramnames
				ft := fd.Type
				for _, f := range ft.Params.List {
					nm := ""
					if len(f.Names) > 0 {
						nm = f.Names[0].String()
						if nm == "_" {
							nm = "_gen"
						}
					}
					fx.paramnames = append(fx.paramnames, nm)
				}
			}
		}
	}
	if false {
		for i, f := range types {
			log.Printf("%d %s %s", i, f.Internal, f.Found)
		}
	}
}

type Function struct {
	Name       string
	Internal   string // first letter lower case
	paramtypes []string
	paramnames []string
	Results    []string
	Param      string
	Result     string // do it in code, easier than in a template
	Invoke     string
	Found      string // file it was found in
}

var types []*Function
var byname = map[string]*Function{} // internal names

func doTypes() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, *def, nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	fd, err := os.Create("/tmp/ast")
	if err != nil {
		log.Fatal(err)
	}
	ast.Fprint(fd, fset, f, ast.NotNilFilter)
	ast.Inspect(f, inter)
	sort.Slice(types, func(i, j int) bool { return types[i].Name < types[j].Name })
	if false {
		for i, f := range types {
			log.Printf("%d %s(%v) %v", i, f.Name, f.paramtypes, f.Results)
		}
	}
}

func inter(n ast.Node) bool {
	x, ok := n.(*ast.TypeSpec)
	if !ok || x.Name.Name != *typ {
		return true
	}
	m := x.Type.(*ast.InterfaceType).Methods.List
	for _, fld := range m {
		fn := fld.Type.(*ast.FuncType)
		p := fn.Params.List
		r := fn.Results.List
		fx := &Function{
			Name: fld.Names[0].String(),
		}
		fx.Internal = strings.ToLower(fx.Name[:1]) + fx.Name[1:]
		for _, f := range p {
			fx.paramtypes = append(fx.paramtypes, whatis(f.Type))
		}
		for _, f := range r {
			fx.Results = append(fx.Results, whatis(f.Type))
		}
		types = append(types, fx)
		byname[fx.Internal] = fx
	}
	return false
}

func whatis(x ast.Expr) string {
	switch n := x.(type) {
	case *ast.SelectorExpr:
		return whatis(n.X) + "." + n.Sel.String()
	case *ast.StarExpr:
		return "*" + whatis(n.X)
	case *ast.Ident:
		if ast.IsExported(n.Name) {
			// these are from package protocol
			return "protocol." + n.Name
		}
		return n.Name
	case *ast.ArrayType:
		return "[]" + whatis(n.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		log.Fatalf("Fatal %T", x)
		return fmt.Sprintf("%T", x)
	}
}
