package noosexit

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer находит прямые вызовы os.Exit в функции main пакета main.
var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "forbid direct os.Exit calls in main function of main package",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Фильтруем только вызовы функций
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspector.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// Проверяем, что это вызов функции
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		// Проверяем, что это вызов os.Exit
		pkg, ok := fun.X.(*ast.Ident)
		if !ok || pkg.Name != "os" || fun.Sel.Name != "Exit" {
			return
		}

		// Проверяем, что мы в пакете main
		if pass.Pkg.Name() != "main" {
			return
		}

		// Ищем родительскую функцию
		for _, file := range pass.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				if fn, ok := node.(*ast.FuncDecl); ok {
					if fn.Name.Name == "main" {
						// Проверяем, находится ли вызов внутри функции main
						if pass.Fset.Position(call.Pos()).Line >= pass.Fset.Position(fn.Pos()).Line &&
							pass.Fset.Position(call.Pos()).Line <= pass.Fset.Position(fn.End()).Line {
							pass.Reportf(call.Pos(), "direct call to os.Exit in main function is forbidden")
						}
					}
				}
				return true
			})
		}
	})

	return nil, nil
}
