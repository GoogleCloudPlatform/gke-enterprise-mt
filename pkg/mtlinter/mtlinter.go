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

		commentInfoByFile := make(map[*token.File]fileCommentInfo, len(pass.Files))
		for _, file := range pass.Files {
			if isTestFile(pass.Fset, file.Pos()) {
				continue
			}
			if tf := pass.Fset.File(file.Pos()); tf != nil {
				commentInfoByFile[tf] = collectFileCommentInfo(pass.Fset, file, pass.Report)
			}
		}

		// 0. Check for forbidden imports (promauto)
		for _, file := range pass.Files {
			if isTestFile(pass.Fset, file.Pos()) {
				continue
			}
			info := commentInfoByFile[pass.Fset.File(file.Pos())]
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.IMPORT {
					continue
				}
				genDeclIgnored := !genDecl.Lparen.IsValid() && hasIgnoreComment(genDecl.Doc)
				for _, spec := range genDecl.Specs {
					imp, ok := spec.(*ast.ImportSpec)
					if !ok {
						continue
					}
					impLine := pass.Fset.Position(imp.Pos()).Line
					if genDeclIgnored || hasIgnoreComment(imp.Doc) || info.sameLineValidIgnore[impLine] {
						continue
					}
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
		}

		// 1. Check for package-level variables
		for _, file := range pass.Files {
			if isTestFile(pass.Fset, file.Pos()) {
				continue
			}
			info := commentInfoByFile[pass.Fset.File(file.Pos())]
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.VAR {
					continue
				}
				if !genDecl.Lparen.IsValid() {
					if hasIgnoreComment(genDecl.Doc) {
						continue
					}
				} else if genDecl.Doc != nil {
					for _, c := range genDecl.Doc.List {
						if isDirective, hasReason := parseIgnoreComment(c); isDirective && hasReason {
							pass.Report(analysis.Diagnostic{
								Pos:     c.Pos(),
								Message: "mtlint:ignore on a var block is not supported; annotate each variable individually",
							})
						}
					}
				}

				for _, spec := range genDecl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					if hasIgnoreComment(valueSpec.Doc) {
						continue
					}

					for _, name := range valueSpec.Names {
						nameLine := pass.Fset.Position(name.Pos()).Line
						if info.sameLineValidIgnore[nameLine] {
							continue
						}
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
			if isTestFile(pass.Fset, n.Pos()) {
				return
			}
			call := n.(*ast.CallExpr)
			info := commentInfoByFile[pass.Fset.File(call.Pos())]
			callLine := pass.Fset.Position(call.Pos()).Line
			if info.sameLineValidIgnore[callLine] || hasPrecedingCallIgnore(info, callLine) {
				return
			}
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
	"third_party/golang/prometheus/client/prometheus/promauto/promauto",
	"google3/third_party/golang/prometheus/client/prometheus/promauto",
	"google3/third_party/golang/prometheus/client/prometheus/promauto/promauto",
}

const ignoreDirective = "mtlint:ignore"

func isTestFile(fset *token.FileSet, pos token.Pos) bool {
	if fset == nil {
		return false
	}
	f := fset.File(pos)
	return f != nil && strings.HasSuffix(f.Name(), "_test.go")
}

type fileCommentInfo struct {
	sameLineValidIgnore   map[int]bool
	standaloneCommentLine map[int]bool
	standaloneValidIgnore map[int]bool
}

func parseIgnoreComment(c *ast.Comment) (isDirective bool, hasReason bool) {
	if c == nil || !strings.HasPrefix(c.Text, "//") {
		return false, false
	}
	s := strings.TrimRight(c.Text[2:], "\r")
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, ignoreDirective) {
		return false, false
	}
	rest := s[len(ignoreDirective):]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return false, false
	}
	return true, strings.TrimSpace(rest) != ""
}

func collectCodeLines(fset *token.FileSet, file *ast.File) map[int]bool {
	codeLines := make(map[int]bool)
	if fset == nil || file == nil {
		return codeLines
	}
	markPos := func(p token.Pos) {
		if p.IsValid() {
			codeLines[fset.Position(p).Line] = true
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		switch x := n.(type) {
		case *ast.Comment, *ast.CommentGroup:
			return false
		case *ast.File:
			markPos(x.Package)
			return true
		case *ast.BasicLit:
			if x.Pos().IsValid() && x.End() > x.Pos() {
				startLine := fset.Position(x.Pos()).Line
				endLine := fset.Position(x.End() - 1).Line
				for line := startLine; line <= endLine; line++ {
					codeLines[line] = true
				}
			}
			return false
		case *ast.GenDecl:
			markPos(x.Lparen)
			markPos(x.Rparen)
		case *ast.CallExpr:
			markPos(x.Lparen)
			markPos(x.Ellipsis)
			markPos(x.Rparen)
		case *ast.AssignStmt:
			markPos(x.TokPos)
		case *ast.BinaryExpr:
			markPos(x.OpPos)
		case *ast.KeyValueExpr:
			markPos(x.Colon)
		case *ast.CaseClause:
			markPos(x.Colon)
		case *ast.CommClause:
			markPos(x.Colon)
		case *ast.TypeSpec:
			markPos(x.Assign)
		}
		markPos(n.Pos())
		if end := n.End(); end.IsValid() && end > n.Pos() {
			markPos(end - 1)
		}
		return true
	})
	return codeLines
}

func collectFileCommentInfo(fset *token.FileSet, file *ast.File, report func(analysis.Diagnostic)) fileCommentInfo {
	info := fileCommentInfo{
		sameLineValidIgnore:   make(map[int]bool),
		standaloneCommentLine: make(map[int]bool),
		standaloneValidIgnore: make(map[int]bool),
	}
	if fset == nil || file == nil {
		return info
	}
	codeLines := collectCodeLines(fset, file)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if !strings.HasPrefix(c.Text, "//") {
				continue
			}
			line := fset.Position(c.Pos()).Line
			standalone := !codeLines[line]
			if standalone {
				info.standaloneCommentLine[line] = true
			}
			isDirective, hasReason := parseIgnoreComment(c)
			if !isDirective {
				continue
			}
			if !hasReason {
				if report != nil {
					report(analysis.Diagnostic{
						Pos:     c.Pos(),
						Message: "mtlint:ignore requires a reason",
					})
				}
				continue
			}
			info.sameLineValidIgnore[line] = true
			if standalone {
				info.standaloneValidIgnore[line] = true
			}
		}
	}
	return info
}

func hasIgnoreComment(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		if isDirective, hasReason := parseIgnoreComment(c); isDirective && hasReason {
			return true
		}
	}
	return false
}

func hasPrecedingCallIgnore(info fileCommentInfo, callLine int) bool {
	for line := callLine - 1; line >= 1 && info.standaloneCommentLine[line]; line-- {
		if info.standaloneValidIgnore[line] {
			return true
		}
	}
	return false
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
