// Package mtlinter implements a static analyzer to enforce GKE Multi-Tenancy
// observability design patterns (no global metrics, no direct prometheus register calls).
package mtlinter

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NewAnalyzer creates a new instance of the GKE Multi-Tenancy metrics linter analyzer.
// Using a constructor instead of a global variable allows tests to run with
// isolated flag states.
func NewAnalyzer() *analysis.Analyzer {
	var checkPackages string
	var excludePackages string

	analyzer := &analysis.Analyzer{
		Name:     "mtmetrics",
		Doc:      "checks for forbidden global prometheus metrics and registration calls in MT environment",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	analyzer.Flags.StringVar(&checkPackages, "check-packages", "", "comma-separated list of package paths to check (supporting ... wildcard)")
	analyzer.Flags.StringVar(&excludePackages, "exclude-packages", "", "comma-separated list of packages to skip")

	analyzer.Run = func(pass *analysis.Pass) (any, error) {
		shouldCheck := false

		if checkPackages != "" {
			if isPackageMatched(pass.Pkg.Path(), checkPackages) {
				shouldCheck = true
			}
		} else {
			// Opt-in check: only run if the package imports mtmetrics
			for _, imp := range pass.Pkg.Imports() {
				for _, mtImp := range mtmetricsImports {
					if imp.Path() == mtImp {
						shouldCheck = true
						break
					}
				}
				if shouldCheck {
					break
				}
			}
		}

		if shouldCheck && excludePackages != "" {
			if isPackageMatched(pass.Pkg.Path(), excludePackages) {
				shouldCheck = false
			}
		}

		if !shouldCheck {
			return nil, nil
		}

		inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

		// 0. Check for forbidden imports (promauto)
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				for _, forbidden := range promautoImports {
					if path == forbidden {
						pass.Report(analysis.Diagnostic{
							Pos:     imp.Pos(),
							Message: "import of promauto is forbidden in MT mode; it registers metrics globally",
						})
					}
				}
			}
		}

		// 1. Check for package-level variables
		for _, file := range pass.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.VAR {
					continue
				}

				for _, spec := range genDecl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for _, name := range valueSpec.Names {
						obj := pass.TypesInfo.Defs[name]
						if obj == nil {
							continue
						}

						if containsPrometheusType(obj.Type(), make(map[string]bool)) {
							pass.Report(analysis.Diagnostic{
								Pos:     name.Pos(),
								Message: "package-level global metric variable is forbidden in MT mode: " + name.Name,
							})
						}
					}
				}
			}
		}

		// 2. Check for forbidden calls (prometheus.Register, prometheus.DefaultRegisterer.Register)
		nodeTypes := []ast.Node{
			(*ast.CallExpr)(nil),
		}
		inspect.Preorder(nodeTypes, func(n ast.Node) {
			call := n.(*ast.CallExpr)
			fun, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return
			}

			isViolation := false
			msg := ""

			// Case A: prom.Register(...)
			if ident, ok := fun.X.(*ast.Ident); ok {
				if isPrometheusPackage(pass, ident) {
					switch fun.Sel.Name {
					case "Register", "MustRegister", "MustRegisterOrDie":
						isViolation = true
						msg = "direct call to prometheus." + fun.Sel.Name + " is forbidden; use mtmetrics factory instead"
					}
				}
			} else if sel, ok := fun.X.(*ast.SelectorExpr); ok {
				// Case B: prom.DefaultRegisterer.Register(...)
				if sel.Sel.Name == "DefaultRegisterer" {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if isPrometheusPackage(pass, ident) {
							switch fun.Sel.Name {
							case "Register", "MustRegister", "MustRegisterOrDie":
								isViolation = true
								msg = "registration to prometheus.DefaultRegisterer is forbidden; use mtmetrics factory instead"
							}
						}
					}
				}
			}

			if isViolation {
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					Message: msg,
				})
			}
		})

		return nil, nil
	}

	return analyzer
}

// Forbidden import paths for prometheus
var prometheusImports = []string{
	"github.com/prometheus/client_golang/prometheus",
	"third_party/golang/prometheus/client/prometheus/prometheus",
	"google3/third_party/golang/prometheus/client/prometheus/prometheus",
}

// Forbidden import paths for promauto
var promautoImports = []string{
	"github.com/prometheus/client_golang/prometheus/promauto",
	"third_party/golang/prometheus/client/prometheus/promauto",
	"google3/third_party/golang/prometheus/client/prometheus/promauto",
}

// Import paths that trigger MT checks (opt-in)
var mtmetricsImports = []string{
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics",
	"google3/third_party/tenancy/components/mtmetrics/mtmetrics",
}

// matchPackage checks if a package path matches a pattern.
// Pattern can end with "..." to match subpackages.
func matchPackage(path, pattern string) bool {
	if pattern == "..." {
		return true
	}
	if strings.HasSuffix(pattern, "...") {
		prefix := strings.TrimSuffix(pattern, "...")
		if strings.HasSuffix(prefix, "/") {
			cleanPrefix := strings.TrimSuffix(prefix, "/")
			return path == cleanPrefix || strings.HasPrefix(path, prefix)
		}
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}

func isPackageMatched(path string, commaSeparatedPatterns string) bool {
	if commaSeparatedPatterns == "" {
		return false
	}
	patterns := strings.Split(commaSeparatedPatterns, ",")
	for _, pattern := range patterns {
		if matchPackage(path, strings.TrimSpace(pattern)) {
			return true
		}
	}
	return false
}

// Helper to check if a type is from prometheus package
func isPrometheusType(t types.Type) bool {
	if t == nil {
		return false
	}
	// Check named types (like prometheus.CounterVec)
	if named, ok := t.(*types.Named); ok {
		pkg := named.Obj().Pkg()
		if pkg != nil {
			for _, imp := range prometheusImports {
				if pkg.Path() == imp {
					return true
				}
			}
		}
	}
	return false
}

// Recursively checks if a type contains a prometheus type (handles pointers, slices, maps, structs, named types)
func containsPrometheusType(t types.Type, visited map[string]bool) bool {
	if t == nil {
		return false
	}

	tStr := t.String()
	if visited[tStr] {
		return false
	}
	visited[tStr] = true
	defer delete(visited, tStr)

	// Dereference pointers
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}

	if isPrometheusType(t) {
		return true
	}

	switch x := t.(type) {
	case *types.Slice:
		return containsPrometheusType(x.Elem(), visited)
	case *types.Array:
		return containsPrometheusType(x.Elem(), visited)
	case *types.Map:
		return containsPrometheusType(x.Key(), visited) || containsPrometheusType(x.Elem(), visited)
	case *types.Chan:
		return containsPrometheusType(x.Elem(), visited)
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			if containsPrometheusType(x.Field(i).Type(), visited) {
				return true
			}
		}
	case *types.Named:
		return containsPrometheusType(x.Underlying(), visited)
	}

	return false
}

func isPrometheusPackage(pass *analysis.Pass, ident *ast.Ident) bool {
	obj := pass.TypesInfo.Uses[ident]
	if obj == nil {
		return false
	}
	pkgName, ok := obj.(*types.PkgName)
	if !ok {
		return false
	}
	for _, imp := range prometheusImports {
		if pkgName.Imported().Path() == imp {
			return true
		}
	}
	return false
}
