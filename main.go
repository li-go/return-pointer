package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	typeSpecs = make(map[string]*ast.TypeSpec)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage:\n\t%s [directory]\n", os.Args[0])
		os.Exit(2)
	}

	inspectFuncs := []inspectFunc{
		inspectTypeSpecs,
		inspectFuncDecls,
	}

	for _, inspectFn := range inspectFuncs {
		if err := filepath.Walk(os.Args[1], func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				return nil
			}

			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}

			for _, pkg := range pkgs {
				inspectFn(fset, pkg)
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
}

type inspectFunc func(fset *token.FileSet, pkg *ast.Package)

func inspectTypeSpecs(fset *token.FileSet, pkg *ast.Package) {
	ast.Inspect(pkg, func(node ast.Node) bool {
		if spec, ok := node.(*ast.TypeSpec); ok {
			typeSpecs[pkg.Name+"."+spec.Name.String()] = spec
		}
		return true
	})
}

func inspectFuncDecls(fset *token.FileSet, pkg *ast.Package) {
	ast.Inspect(pkg, func(node ast.Node) bool {
		if fn, ok := node.(*ast.FuncDecl); ok && testFuncDecl(fn) {
			s, err := nodeStr(fn)
			if err != nil {
				log.Printf("nodeStr: %v", err)
				return false
			}
			fnStr := strings.Split(s, "\n")[0]
			fnStr = fnStr[:len(fnStr)-2]
			file := fset.File(fn.Pos())
			fmt.Printf("%s:%d %s\n", file.Name(), file.Line(fn.Pos()), fnStr)
		}
		return true
	})
}

func testFuncDecl(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if testField(field) {
			return true
		}
	}
	return false
}

func testField(field *ast.Field) bool {
	typ := overlayType(field.Type)
	_, ok := typ.(*ast.StructType)
	return ok
}

func overlayType(typ ast.Expr) ast.Expr {
	switch expr := typ.(type) {
	case *ast.Ident:
		if expr.Obj != nil {
			if typeSpec, ok := expr.Obj.Decl.(*ast.TypeSpec); ok {
				return overlayType(typeSpec.Type)
			}
		}
		return expr
	case *ast.SelectorExpr:
		name := fmt.Sprintf("%s.%s", expr.X, expr.Sel)
		if typeSpec, ok := typeSpecs[name]; ok {
			return overlayType(typeSpec.Type)
		}
		return expr
	default:
		return typ
	}
}

func nodeStr(node ast.Node) (string, error) {
	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", err
	}
	return buf.String(), nil
}
