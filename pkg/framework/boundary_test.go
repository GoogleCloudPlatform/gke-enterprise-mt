package framework

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDependencyBoundary parses all Go files in the framework package to ensure
// that concrete GKE multitenancy API packages are never imported directly.
// The framework must remain purely generic, operating on unstructured objects via dynamic clients.
// This boundary enforcement runs automatically in TAP presubmits.
func TestDependencyBoundary(t *testing.T) {
	forbiddenSubstrings := []string{
		"google3/cloud/kubernetes/tenancy/apis",
		"google3/third_party/tenancy/apis",
	}

	srcDir := os.Getenv("TEST_SRCDIR")
	workspace := os.Getenv("TEST_WORKSPACE")
	pattern := "*.go"
	if srcDir != "" && workspace != "" {
		pattern = filepath.Join(srcDir, workspace, "third_party/tenancy/components/framework", "*.go")
	}

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("failed to glob .go files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no .go source files found to verify")
	}

	for _, filename := range files {
		// Do not verify boundary_test.go itself to avoid self-matching literal strings
		if filepath.Base(filename) == "boundary_test.go" {
			continue
		}

		src, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("failed to read source file %s: %v", filename, err)
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filename, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse file %s: %v", filename, err)
		}

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenSubstrings {
				if strings.Contains(importPath, forbidden) {
					t.Errorf("File %s violates dependency boundary by importing forbidden API package: %s", filename, importPath)
				}
			}
		}
	}
}
