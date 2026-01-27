package main

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
	"text/template"
)

// Stores information about structures that need reset
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

	// Skip unnecessary dirs
	if d.IsDir() && (d.Name() == ".git" || d.Name() == "profiles") {
		return filepath.SkipDir
	}

	// Process only Go files
	if !d.IsDir() && isGoFile(d.Name()) {
		log.Printf("Processing file: %s", path)

		// Parse Go file to find structures with generate:reset comment
		structs, err := parseGoFile(path)
		if err != nil {
			log.Printf("Failed to parse file %s: %v", path, err)
			return nil
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

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		// Check for the presence of the generate:reset comment
		if hasResetComment(genDecl.Doc) {
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

// Checks for the presence of the generate:reset comment
func hasResetComment(commentGroup *ast.CommentGroup) bool {
	if commentGroup == nil {
		return false
	}

	for _, comment := range commentGroup.List {
		if strings.Contains(comment.Text, "generate:reset") {
			return true
		}
	}

	return false
}

// MethodTemplateData holds data for the method template
type MethodTemplateData struct {
	StructName string
	Fields     []string
}

// generateResetMethod generates the Reset() method code for the specified structure
func generateResetMethod(structName string, structType *ast.StructType) string {
	// Define the template for the method
	methodTemplate := `// Reset resets all field values of structure {{.StructName}} to zero value
func (s *{{.StructName}}) Reset() {
	if s == nil {
		return
	}
{{range .Fields}}	{{.}}
{{end}}}
`

	// Prepare field reset codes
	var fields []string
	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			if len(field.Names) > 0 && field.Names[0] != nil {
				fieldName := field.Names[0].Name
				resetCode := getResetCode(fieldName, field.Type)
				// Remove trailing newline as template will handle formatting
				resetCode = strings.TrimSuffix(resetCode, "\n")
				// Remove leading tabulation if present
				resetCode = strings.TrimPrefix(resetCode, "\t")
				fields = append(fields, resetCode)
			}
		}
	}

	// Execute the template
	tmpl, err := template.New("method").Parse(methodTemplate)
	if err != nil {
		log.Printf("Failed to parse template: %v", err)
		return ""
	}

	var builder strings.Builder
	data := MethodTemplateData{
		StructName: structName,
		Fields:     fields,
	}

	err = tmpl.Execute(&builder, data)
	if err != nil {
		log.Printf("Failed to execute template: %v", err)
		return ""
	}

	return builder.String()
}

// getResetCode returns the reset code for the specified field
func getResetCode(fieldName string, expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		// Simple types
		if zeroValue, exists := builtInTypeZeroValues[t.Name]; exists {
			return fmt.Sprintf("\ts.%s = %s\n", fieldName, zeroValue)
		}
		// For custom types
		return fmt.Sprintf("\tif resetter, ok := s.%s.(interface{ Reset() }); ok {\n\t\tresetter.Reset()\n\t}\n", fieldName)
	case *ast.StarExpr:
		// Pointers
		pointedType := formatNode(t.X)
		if isBuiltInType(pointedType) {
			zeroValue := getZeroValueForType(pointedType)
			return fmt.Sprintf("\tif s.%s != nil {\n\t\t*s.%s = %s\n\t}\n", fieldName, fieldName, zeroValue)
		}
		return fmt.Sprintf("\tif s.%s != nil {\n\t\ts.%s.Reset()\n\t}\n", fieldName, fieldName)
	case *ast.ArrayType:
		if t.Len == nil {
			// Slice - truncate to length 0
			return fmt.Sprintf("\ts.%s = s.%s[:0]\n", fieldName, fieldName)
		} else {
			// Array - reset each element to zero value
			elementType := formatNode(t.Elt)
			if elementType != "" {
				return fmt.Sprintf("\tfor i := range s.%s {\n\t\tvar zeroValue %s\n\t\ts.%s[i] = zeroValue\n\t}\n", fieldName, elementType, fieldName)
			} else {
				return fmt.Sprintf("\ts.%s = nil\n", fieldName)
			}
		}
	case *ast.MapType:
		// Maps - use clear function
		return fmt.Sprintf("\tclear(s.%s)\n", fieldName)
	case *ast.StructType:
		return fmt.Sprintf("\tif resetter, ok := s.%s.(interface{ Reset() }); ok {\n\t\tresetter.Reset()\n\t}\n", fieldName)
	case *ast.ChanType:
		// Channels
		return fmt.Sprintf("\ts.%s = nil\n", fieldName)
	case *ast.FuncType:
		// Functions
		return fmt.Sprintf("\ts.%s = nil\n", fieldName)
	default:
		// For all other types, use interface check
		return fmt.Sprintf("\tif resetter, ok := s.%s.(interface{ Reset() }); ok {\n\t\tresetter.Reset()\n\t}\n", fieldName)
	}
}

// formatNode converts an ast.Node to its string representation
func formatNode(node ast.Node) string {
	var buf bytes.Buffer
	err := format.Node(&buf, token.NewFileSet(), node)
	if err != nil {
		return ""
	}
	return buf.String()
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

// generateAllResetMethods generates Reset() methods for all found structures
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

		// Create a builder for generating method code
		var builder strings.Builder

		// Write the file header
		builder.WriteString("// Code generated by reset generator. DO NOT EDIT.\n\n")
		builder.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

		// Generate Reset() methods for each structure
		for i, structInfo := range packageStructs.Structs {
			if i > 0 {
				builder.WriteString("\n")
			}

			methodCode := generateResetMethod(structInfo.Name, structInfo.Type)
			builder.WriteString(methodCode)
		}

		// Format the generated code
		formatted, err := format.Source([]byte(builder.String()))
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

// builtInTypeZeroValues maps built-in Go types to their zero values
var builtInTypeZeroValues = map[string]string{
	"bool":      "false",
	"string":    "\"\"",
	"int":       "0",
	"int32":     "0",
	"int64":     "0",
	"uint":      "0",
	"uint32":    "0",
	"uint64":    "0",
	"float32":   "0.0",
	"float64":   "0.0",
	"complex64": "0i",
	"byte":      "0",
}

// isBuiltInType checks if a type is a built-in Go type
func isBuiltInType(typeName string) bool {
	_, exists := builtInTypeZeroValues[typeName]
	return exists
}

// getZeroValueForType returns the zero value for a built-in type as a string
func getZeroValueForType(typeName string) string {
	if zeroValue, ok := builtInTypeZeroValues[typeName]; ok {
		return zeroValue
	}
	return "nil"
}
