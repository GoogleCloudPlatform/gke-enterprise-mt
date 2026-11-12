package mtlinter

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// Testing is a minimal subset of *testing.T used by CheckDir to report lint
// violations and setup errors.
type Testing interface {
	Errorf(format string, args ...any)
}

// Option configures CheckDir execution.
type Option func(*checkConfig)

type checkConfig struct {
	checkPackages   string
	excludePackages string
}

// WithCheckPackages configures comma-separated package patterns to check
// (supporting "..." wildcards), overriding the default mtmetrics-import opt-in.
func WithCheckPackages(patterns ...string) Option {
	return func(c *checkConfig) {
		c.checkPackages = strings.Join(patterns, ",")
	}
}

// WithExcludePackages configures comma-separated package patterns to skip
// (supporting "..." wildcards).
func WithExcludePackages(patterns ...string) Option {
	return func(c *checkConfig) {
		c.excludePackages = strings.Join(patterns, ",")
	}
}

// CheckDir runs the mtlinter static analyzer in-process over all non-test Go
// packages rooted at dir, reporting any violations via t.Errorf and returning
// the formatted diagnostic messages.
//
// A relative dir is resolved against the current working directory, falling
// back to the runfiles workspace under $TEST_SRCDIR. In Blaze tests the
// working directory is the google3 runfiles root, so pass a google3-relative
// path such as "cloud/kubernetes/foo" rather than ".". Patterns passed to
// WithCheckPackages and WithExcludePackages match package paths relative to
// dir.
//
// Unlike analysistest.Run, CheckDir uses an in-process type importer and does
// not invoke the external "go list" command, allowing it to run inside hermetic
// Blaze go_test sandboxes.
func CheckDir(t Testing, dir string, opts ...Option) []string {
	var cfg checkConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	analyzer := NewAnalyzer()
	if cfg.checkPackages != "" {
		if err := analyzer.Flags.Set("check-packages", cfg.checkPackages); err != nil {
			t.Errorf("failed to set check-packages flag: %v", err)
			return nil
		}
	}
	if cfg.excludePackages != "" {
		if err := analyzer.Flags.Set("exclude-packages", cfg.excludePackages); err != nil {
			t.Errorf("failed to set exclude-packages flag: %v", err)
			return nil
		}
	}

	rootDir := resolveDir(dir)
	if rootDir == "" {
		t.Errorf("directory %q not found", dir)
		return nil
	}

	var allDiagnostics []string
	totalFiles := 0

	walkErr := filepath.WalkDir(rootDir, func(currentDir string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if currentDir != rootDir && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return err
		}

		var goFiles []string
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			goFiles = append(goFiles, filepath.Join(currentDir, name))
		}
		if len(goFiles) == 0 {
			return nil
		}

		fset := token.NewFileSet()
		byPkg := make(map[string][]*ast.File)
		var pkgNames []string
		for _, filePath := range goFiles {
			fileAst, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
			if err != nil {
				t.Errorf("failed to parse %s: %v", filePath, err)
				continue
			}
			totalFiles++
			pkgName := fileAst.Name.Name
			if _, exists := byPkg[pkgName]; !exists {
				pkgNames = append(pkgNames, pkgName)
			}
			byPkg[pkgName] = append(byPkg[pkgName], fileAst)
		}
		sort.Strings(pkgNames)

		relDir, err := filepath.Rel(rootDir, currentDir)
		if err != nil {
			return err
		}
		relDir = filepath.ToSlash(relDir)

		for _, pkgName := range pkgNames {
			files := byPkg[pkgName]
			pkgPath := relDir
			if relDir == "." {
				pkgPath = pkgName
			} else if len(pkgNames) > 1 && filepath.Base(relDir) != pkgName {
				pkgPath = relDir + "/" + pkgName
			}

			diags, err := runOnPackage(fset, pkgPath, files, analyzer)
			if err != nil {
				t.Errorf("failed to analyze package %s in %s: %v", pkgPath, currentDir, err)
				continue
			}
			for _, diag := range diags {
				allDiagnostics = append(allDiagnostics, diag)
				t.Errorf("%s", diag)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Errorf("failed walking %s: %v", rootDir, walkErr)
		return allDiagnostics
	}
	if totalFiles == 0 {
		t.Errorf("no non-test Go source files found in %s", rootDir)
	}
	return allDiagnostics
}

func resolveDir(dir string) string {
	if isDir(dir) {
		return dir
	}
	srcDir := os.Getenv("TEST_SRCDIR")
	if filepath.IsAbs(dir) || srcDir == "" {
		return ""
	}
	if ws := os.Getenv("TEST_WORKSPACE"); ws != "" {
		if cand := filepath.Join(srcDir, ws, dir); isDir(cand) {
			return cand
		}
	}
	if cand := filepath.Join(srcDir, "google3", dir); isDir(cand) {
		return cand
	}
	return ""
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func runOnPackage(fset *token.FileSet, pkgPath string, files []*ast.File, analyzer *analysis.Analyzer) ([]string, error) {
	imp := newSyntheticImporter(fset)
	conf := types.Config{
		Importer: imp,
		Error:    func(error) {},
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, _ := conf.Check(pkgPath, fset, files, info)
	if pkg == nil {
		return nil, fmt.Errorf("typecheck returned nil package for %s", pkgPath)
	}

	passInspect := &analysis.Pass{
		Analyzer:  inspect.Analyzer,
		Fset:      fset,
		Files:     files,
		Pkg:       pkg,
		TypesInfo: info,
	}
	inspectorObj, err := inspect.Analyzer.Run(passInspect)
	if err != nil {
		return nil, fmt.Errorf("inspect analyzer failed: %w", err)
	}

	var diags []string
	pass := &analysis.Pass{
		Analyzer:  analyzer,
		Fset:      fset,
		Files:     files,
		Pkg:       pkg,
		TypesInfo: info,
		ResultOf: map[*analysis.Analyzer]any{
			inspect.Analyzer: inspectorObj,
		},
		Report: func(d analysis.Diagnostic) {
			pos := fset.Position(d.Pos)
			diags = append(diags, fmt.Sprintf("%s: %s", pos, d.Message))
		},
	}
	if _, err := analyzer.Run(pass); err != nil {
		return nil, err
	}
	return diags, nil
}

//go:embed prometheus_stub.txt
var syntheticPrometheusSrc string

type syntheticImporter struct {
	fset *token.FileSet
	pkgs map[string]*types.Package
}

func newSyntheticImporter(fset *token.FileSet) *syntheticImporter {
	return &syntheticImporter{
		fset: fset,
		pkgs: make(map[string]*types.Package),
	}
}

func (s *syntheticImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := s.pkgs[path]; ok {
		return pkg, nil
	}
	for _, promPath := range prometheusImports {
		if path == promPath {
			fileAst, err := parser.ParseFile(s.fset, "prometheus_stub.go", syntheticPrometheusSrc, 0)
			if err != nil {
				continue
			}
			conf := types.Config{
				Importer: s,
				Error:    func(error) {},
			}
			pkg, _ := conf.Check(path, s.fset, []*ast.File{fileAst}, nil)
			if pkg != nil {
				s.pkgs[path] = pkg
				return pkg, nil
			}
		}
	}
	pkg := types.NewPackage(path, filepath.Base(path))
	pkg.MarkComplete()
	s.pkgs[path] = pkg
	return pkg, nil
}
