// Package main implements a static analysis tool that checks for:
// 1. Usage of built-in panic() function anywhere in the code
// 2. Usage of log.Fatal()/log.Fatalf()/log.Fatalln() or os.Exit() outside of main function in main package
package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the main analyzer for detecting improper usage of panic and exit functions
var Analyzer = &analysis.Analyzer{
	Name: "panicexit",
	Doc:  "reports usage of panic and log.Fatal/os.Exit outside of main function in main package",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

// run executes the analysis logic
func run(pass *analysis.Pass) (interface{}, error) {
	// Get the inspector from the required analyzer
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Define which AST node types we want to inspect
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil), // Function calls
		(*ast.FuncDecl)(nil), // Function declarations
	}

	// Variable to track if we're currently inside the main function of main package
	inMain := false

	// Preorder traversal of AST nodes
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// Check if this is the main function in the main package
			if pass.Pkg.Name() == "main" && node.Name.Name == "main" {
				inMain = true
			} else {
				// For all other functions, we're not in main
				inMain = false
			}
		case *ast.CallExpr:
			// Check for direct panic() calls
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				pass.Reportf(ident.Pos(), "found usage of panic")
			}

			// Check for log.Fatal() or os.Exit() calls outside main function
			if !inMain {
				if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						switch ident.Name + "." + sel.Sel.Name {
						case "log.Fatal", "log.Fatalf", "log.Fatalln":
							pass.Reportf(node.Pos(), "found usage of %s outside of main function", ident.Name+"."+sel.Sel.Name)
						case "os.Exit":
							pass.Reportf(node.Pos(), "found usage of %s outside of main function", ident.Name+"."+sel.Sel.Name)
						}
					}
				}
			}
		}
	})

	return nil, nil
}
