package osexitcheck

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "osexitcheck",
	Doc:  "check for os.Exit() call, which isn't allowed",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.CallExpr:

				if selectorExpr, ok := x.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := selectorExpr.X.(*ast.Ident); ok {
						if ident.Name == "os" && selectorExpr.Sel.Name == "Exit" {
							pass.Reportf(ident.Pos(), "os.Exit() is disallowed")
						}
					}
				}
			}
			return true
		})
	}
	return nil, nil
}
