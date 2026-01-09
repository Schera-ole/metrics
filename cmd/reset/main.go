package reset

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PackageStructs stores information about structures that need reset for each package
type PackageStructs struct {
	FilePaths []string      // Paths to package files with found structures
	Structs   []*StructInfo // Found structures
}

var packageStructsMap = make(map[string]*PackageStructs) // Map of packages with structure information

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get the current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Determine the project root directory (parent directory of metrics)
	projectRoot := filepath.Join(wd, "..", "..")

	log.Printf("Scanning packages in project root: %s", projectRoot)

	// Scan all packages in the project
	if err := filepath.WalkDir(projectRoot, scanPackage); err != nil {
		return err
	}

	// Generate Reset() methods for all found structures
	return generateAllResetMethods()
}

// scanPackage processes each file during scanning
func scanPackage(path string, d fs.DirEntry, err error) error {
	if err != nil {
		log.Printf("Error accessing path %s: %v", path, err)
		return err
	}

	// Skip vendor directories and other service directories
	if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules") {
		return filepath.SkipDir
	}

	// Process only Go files
	if !d.IsDir() && isGoFile(d.Name()) {
		log.Printf("Processing file: %s", path)

		// Parse Go file to find structures with // generate:reset comment
		structs, err := parseGoFile(path)
		if err != nil {
			log.Printf("Failed to parse file %s: %v", path, err)
			return nil // Continue processing other files
		}

		// If we found structures with the comment, save information about them
		if len(structs) > 0 {
			log.Printf("Found %d struct(s) with // generate:reset comment in %s", len(structs), path)

			// Get the file directory (package)
			dir := filepath.Dir(path)

			// If there is no entry for this package yet, create it
			if packageStructsMap[dir] == nil {
				packageStructsMap[dir] = &PackageStructs{
					FilePaths: []string{},
					Structs:   []*StructInfo{},
				}
			}

			// Add the file path to the package file list
			packageStructsMap[dir].FilePaths = append(packageStructsMap[dir].FilePaths, path)

			// Add found structures
			packageStructsMap[dir].Structs = append(packageStructsMap[dir].Structs, structs...)
		}
	}

	return nil
}

// isGoFile checks if a file is a Go file
func isGoFile(name string) bool {
	return filepath.Ext(name) == ".go" &&
		name != "reset.gen.go"
}

// StructInfo contains information about a structure for generating the Reset method
type StructInfo struct {
	Name string
	Type *ast.StructType
}

// parseGoFile parses a Go file and returns a list of structures with the // generate:reset comment
func parseGoFile(filePath string) ([]*StructInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var resetStructs []*StructInfo

	// Go through all declarations in the file
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		// Check for the presence of the // generate:reset comment
		if hasResetComment(genDecl.Doc) {
			// Look for structures in the type declaration
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				// Save information about the structure
				resetStructs = append(resetStructs, &StructInfo{
					Name: typeSpec.Name.Name,
					Type: structType,
				})
			}
		}
	}

	return resetStructs, nil
}

// hasResetComment checks for the presence of the // generate:reset comment
func hasResetComment(commentGroup *ast.CommentGroup) bool {
	if commentGroup == nil {
		return false
	}

	// Go through all comments in the group
	for _, comment := range commentGroup.List {
		// Check if the comment contains the text "// generate:reset"
		if strings.Contains(comment.Text, "// generate:reset") {
			return true
		}
	}

	return false
}

// generateResetMethod generates the Reset() method code for the specified structure
func generateResetMethod(structName string, structType *ast.StructType) string {
	var buf bytes.Buffer

	// Write the method signature
	buf.WriteString(fmt.Sprintf("// Reset resets all field values of structure %s to zero value\n", structName))
	buf.WriteString(fmt.Sprintf("func (s *%s) Reset() {\n", structName))

	// Generate the Reset() method body
	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			if len(field.Names) > 0 && field.Names[0] != nil {
				fieldName := field.Names[0].Name

				// Determine the zero value for the field depending on its type
				zeroValue := getZeroValue(field.Type)
				buf.WriteString(fmt.Sprintf("\ts.%s = %s\n", fieldName, zeroValue))
			}
		}
	}

	buf.WriteString("}\n")

	return buf.String()
}

// getZeroValue returns the zero value for the specified type
func getZeroValue(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		// Simple types
		switch t.Name {
		case "bool":
			return "false"
		case "string":
			return "\"\""
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
			return "0"
		case "float32", "float64":
			return "0.0"
		case "complex64", "complex128":
			return "0i"
		default:
			// For custom types, return nil
			return "nil"
		}
	case *ast.StarExpr:
		// Pointers
		return "nil"
	case *ast.ArrayType:
		// Arrays and slices
		return "nil"
	case *ast.MapType:
		// Maps
		return "nil"
	case *ast.ChanType:
		// Channels
		return "nil"
	case *ast.FuncType:
		// Functions
		return "nil"
	default:
		// For all other types, return nil
		return "nil"
	}
}

// getPackageName gets the package name from a Go file
func getPackageName(filePath string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", err
	}

	return node.Name.Name, nil
}

// generateAllResetMethods generates Reset() methods for all found structures by packages
func generateAllResetMethods() error {
	// Go through all packages with found structures
	for dir, packageStructs := range packageStructsMap {
		if len(packageStructs.Structs) == 0 {
			continue
		}

		log.Printf("Generating reset methods for %d struct(s) in package %s", len(packageStructs.Structs), dir)

		// Path to the reset.gen.go file
		outputPath := filepath.Join(dir, "reset.gen.go")

		// Get the package name from the first found Go file in the directory
		if len(packageStructs.FilePaths) == 0 {
			log.Printf("No files found for package %s", dir)
			continue
		}

		pkgName, err := getPackageName(packageStructs.FilePaths[0])
		if err != nil {
			log.Printf("Failed to get package name for %s: %v", dir, err)
			continue
		}

		// Create a buffer for generating method code
		var buf bytes.Buffer

		// Write the file header
		buf.WriteString("// Code generated by reset generator. DO NOT EDIT.\n\n")
		buf.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

		// Generate Reset() methods for each structure
		for i, structInfo := range packageStructs.Structs {
			if i > 0 {
				buf.WriteString("\n")
			}

			// Generate the Reset() method for the structure
			methodCode := generateResetMethod(structInfo.Name, structInfo.Type)
			buf.WriteString(methodCode)
		}

		// Format the generated code
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			log.Printf("Failed to format generated code for package %s: %v", dir, err)
			continue
		}

		// Write the generated code to the reset.gen.go file
		log.Printf("Writing generated code to %s", outputPath)
		if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
			log.Printf("Failed to write to reset.gen.go for package %s: %v", dir, err)
			continue
		}

		log.Printf("Generated reset methods for %d struct(s) in package %s", len(packageStructs.Structs), dir)
	}

	return nil
}
