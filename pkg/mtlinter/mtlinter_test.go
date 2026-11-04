package mtlinter_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtlinter"
)

func TestLinter(t *testing.T) {
	testdata := analysistest.TestData()

	// 1. Default opt-in behavior.
	t.Run("default_opt_in", func(t *testing.T) {
		analyzer := mtlinter.NewAnalyzer()
		// optin_violation: imports mtmetrics -> checked, fails.
		// optin_clean: imports mtmetrics -> checked, passes.
		// optin_ignored: no import -> not checked, passes (despite violations).
		analysistest.Run(t, testdata, analyzer, "optin_violation", "optin_clean", "optin_ignored")
	})

	// 2. Explicitly check package via flag.
	t.Run("check-packages", func(t *testing.T) {
		analyzer := mtlinter.NewAnalyzer()
		if err := analyzer.Flags.Set("check-packages", "flag_checked"); err != nil {
			t.Fatalf("failed to set check-packages: %v", err)
		}
		// flag_checked: no import -> checked via flag, fails.
		analysistest.Run(t, testdata, analyzer, "flag_checked")
	})

	// 3. Exclude package via flag.
	t.Run("exclude-packages", func(t *testing.T) {
		analyzer := mtlinter.NewAnalyzer()
		if err := analyzer.Flags.Set("exclude-packages", "flag_excluded"); err != nil {
			t.Fatalf("failed to set exclude-packages: %v", err)
		}
		// flag_excluded: imports mtmetrics -> would check, but excluded via flag, passes.
		analysistest.Run(t, testdata, analyzer, "flag_excluded")
	})

	// 4. Wildcard check-packages and exclude-packages.
	t.Run("wildcard", func(t *testing.T) {
		analyzer := mtlinter.NewAnalyzer()
		if err := analyzer.Flags.Set("check-packages", "wildcard_checked/... "); err != nil {
			t.Fatalf("failed to set check-packages: %v", err)
		}
		if err := analyzer.Flags.Set("exclude-packages", " wildcard_excluded/... "); err != nil {
			t.Fatalf("failed to set exclude-packages: %v", err)
		}
		// wildcard_checked/pkg1: no import -> checked via wildcard, fails.
		// wildcard_excluded/pkg2: imports mtmetrics -> excluded via wildcard, passes.
		analysistest.Run(t, testdata, analyzer, "wildcard_checked/pkg1", "wildcard_excluded/pkg2")
	})

	// 5. Google3 promauto/promauto import path.
	t.Run("google3_promauto_import", func(t *testing.T) {
		files := map[string]string{
			"third_party/golang/prometheus/client/prometheus/promauto/promauto/promauto.go": "package promauto\n",
			"pkg/pkg.go": `package pkg

import _ "third_party/golang/prometheus/client/prometheus/promauto/promauto" // want "import of promauto is forbidden in MT mode; it registers metrics globally"
`,
		}
		dir, cleanup, err := analysistest.WriteFiles(files)
		if err != nil {
			t.Fatalf("WriteFiles failed: %v", err)
		}
		defer cleanup()

		analyzer := mtlinter.NewAnalyzer()
		if err := analyzer.Flags.Set("check-packages", "pkg"); err != nil {
			t.Fatalf("failed to set check-packages: %v", err)
		}
		analysistest.Run(t, dir, analyzer, "pkg")
	})
}

func mockVendorFiles() map[string]string {
	return map[string]string{
		"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics/mtmetrics.go": "package mtmetrics\n",
		"github.com/prometheus/client_golang/prometheus/prometheus.go": `package prometheus

type Counter any
type CounterOpts struct{ Name string }

func NewCounter(CounterOpts) Counter { return nil }
func MustRegister(...any)            {}
func Register(any) error             { return nil }

type Registerer struct{}

func (Registerer) MustRegister(...any) {}
func (Registerer) Register(any) error  { return nil }

var DefaultRegisterer Registerer
`,
		"github.com/prometheus/client_golang/prometheus/promauto/promauto.go": "package promauto\n",
	}
}

func runAnalyzerOnFiles(t *testing.T, extraFiles map[string]string, pkgPatterns ...string) {
	t.Helper()
	files := mockVendorFiles()
	for k, v := range extraFiles {
		files[k] = v
	}
	dir, cleanup, err := analysistest.WriteFiles(files)
	if err != nil {
		t.Fatalf("WriteFiles failed: %v", err)
	}
	t.Cleanup(cleanup)

	analyzer := mtlinter.NewAnalyzer()
	analysistest.Run(t, dir, analyzer, pkgPatterns...)
}

func TestIgnoreDirectiveValidation(t *testing.T) {
	t.Run("valid_suppressions", func(t *testing.T) {
		runAnalyzerOnFiles(t, map[string]string{
			"valid/imports_single_doc.go": `package valid

import _ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"

// mtlint:ignore legacy single import with doc comment
import _ "github.com/prometheus/client_golang/prometheus/promauto"
`,
			"valid/imports_single_sameline.go": `package valid

import _ "github.com/prometheus/client_golang/prometheus/promauto" //mtlint:ignore legacy single import on same line
`,
			"valid/metrics.go": `package valid

import (
	"github.com/prometheus/client_golang/prometheus"
	// mtlint:ignore legacy grouped import with doc comment
	_ "github.com/prometheus/client_golang/prometheus/promauto"
	_ "github.com/prometheus/client_golang/prometheus/promauto" // mtlint:ignore legacy grouped import on same line
)

// mtlint:ignore process-level single var with doc comment
var singleDocCounter prometheus.Counter

var singleSameLineCounter prometheus.Counter //mtlint:ignore process-level single var without space after slashes

var (
	groupedSameLineCounter prometheus.Counter // mtlint:ignore process-level metric
	groupedOpeningCounter  = prometheus.NewCounter( // mtlint:ignore process-level multiline constructor
		prometheus.CounterOpts{
			Name: "grouped_opening_counter",
		},
	)
	// Context line before directive.
	// mtlint:ignore process-level metric with multi-line doc comment
	groupedDocCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "grouped_doc_counter",
		},
	)
	// mtlint:ignore process-level metric followed by explanation
	// Additional explanation line in the same comment group.
	groupedDocCounterTrailingNote prometheus.Counter
)

func init() {
	prometheus.MustRegister(singleDocCounter)      // mtlint:ignore process-level registration
	prometheus.Register(singleSameLineCounter)     //mtlint:ignore process-level registration without space
	prometheus.MustRegister( // mtlint:ignore multiline call with opening-line directive
		groupedSameLineCounter,
	)

	// mtlint:ignore standalone doc comment before MustRegister
	prometheus.MustRegister(groupedOpeningCounter)

	// mtlint:ignore standalone multi-line doc comment before DefaultRegisterer.MustRegister
	// Extra context immediately above the call.
	prometheus.DefaultRegisterer.MustRegister(groupedDocCounter, groupedDocCounterTrailingNote)
}
`,
		}, "valid")
	})

	t.Run("missing_reason_directive", func(t *testing.T) {
		runAnalyzerOnFiles(t, map[string]string{
			"missingreason/pkg.go": `package missingreason

import (
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

/* want +1 "mtlint:ignore requires a reason" */
// mtlint:ignore
var bareDocCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: bareDocCounter"

var bareNoSpaceCounter prometheus.Counter /* want "mtlint:ignore requires a reason" "package-level global metric variable is forbidden in MT mode: bareNoSpaceCounter" */ //mtlint:ignore

/* want +1 "mtlint:ignore requires a reason" */
// mtlint:ignore
var cleanNonMetric int

func init() {
	prometheus.MustRegister(bareDocCounter) /* want "mtlint:ignore requires a reason" "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead" */ // mtlint:ignore   	 

	/* want +1 "mtlint:ignore requires a reason" */
	// mtlint:ignore
	prometheus.Register(bareNoSpaceCounter) // want "direct call to prometheus.Register is forbidden; use mtmetrics factory instead"
}
`,
		}, "missingreason")
	})

	t.Run("block_and_prose_comments_ignored", func(t *testing.T) {
		runAnalyzerOnFiles(t, map[string]string{
			"prose/pkg.go": `package prose

import (
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
	/* mtlint:ignore block comment on import */
	_ "github.com/prometheus/client_golang/prometheus/promauto" // want "import of promauto is forbidden in MT mode; it registers metrics globally"
)

/* mtlint:ignore */
var cleanWithBlockComment int

/* mtlint:ignore legacy block comment */
var blockCommentCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: blockCommentCounter"

// Do not use mtlint:ignore for tenant metrics.
var proseDocCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: proseDocCounter"

// mtlint:ignored non-boundary token prefix
var nonBoundaryCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: nonBoundaryCounter"

func init() {
	/* mtlint:ignore legacy block comment */
	prometheus.MustRegister(blockCommentCounter) // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
	// Do not use mtlint:ignore here either.
	prometheus.Register(proseDocCounter) // want "direct call to prometheus.Register is forbidden; use mtmetrics factory instead"
}
`,
		}, "prose")
	})

	t.Run("scoping_multiline_and_adjacent_lines", func(t *testing.T) {
		runAnalyzerOnFiles(t, map[string]string{
			"scoping/pkg.go": `package scoping

import (
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	innerFieldCounter = prometheus.NewCounter( // want "package-level global metric variable is forbidden in MT mode: innerFieldCounter"
		prometheus.CounterOpts{
			Name: "inner", // mtlint:ignore inner field line must not suppress ValueSpec
		},
	)
	closingLineCounter = prometheus.NewCounter( // want "package-level global metric variable is forbidden in MT mode: closingLineCounter"
		prometheus.CounterOpts{
			Name: "closing",
		},
	) // mtlint:ignore closing paren line must not suppress ValueSpec

	// mtlint:ignore detached by blank line

	detachedVarCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: detachedVarCounter"
)

func helper(int) {}

func init() {
	prometheus.MustRegister( // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
		innerFieldCounter, // mtlint:ignore inner argument line must not suppress CallExpr
	)
	prometheus.MustRegister( // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
		closingLineCounter,
	) // mtlint:ignore closing paren line must not suppress CallExpr

	x := 1 // mtlint:ignore trailing comment on previous code line
	prometheus.MustRegister(innerFieldCounter) // want "direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead"
	_ = x

	helper(
		1,
	) // mtlint:ignore trailing comment on multiline call closing paren
	prometheus.Register(innerFieldCounter) // want "direct call to prometheus.Register is forbidden; use mtmetrics factory instead"

	// mtlint:ignore detached by blank line

	prometheus.DefaultRegisterer.MustRegister(detachedVarCounter) // want "registration to prometheus.DefaultRegisterer is forbidden; use mtmetrics factory instead"
}
`,
		}, "scoping")
	})

	t.Run("parenthesized_block_comments", func(t *testing.T) {
		runAnalyzerOnFiles(t, map[string]string{
			"blocks/pkg.go": `package blocks

// mtlint:ignore import block doc comment does not exempt specs
import (
	_ "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtmetrics"
	"github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/client_golang/prometheus/promauto" // want "import of promauto is forbidden in MT mode; it registers metrics globally"
)

/* want +1 "mtlint:ignore on a var block is not supported; annotate each variable individually" */
// mtlint:ignore var block suppression is rejected
var (
	unannotatedBlockCounter prometheus.Counter // want "package-level global metric variable is forbidden in MT mode: unannotatedBlockCounter"
	// mtlint:ignore per-spec annotation inside the block still works
	annotatedBlockCounter prometheus.Counter
)
`,
		}, "blocks")
	})
}
