package mtlinter_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtlinter"
)

type recordingT struct {
	errors []string
}

func (r *recordingT) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestCheckDirCleanAndIgnored(t *testing.T) {
	srcRoot := filepath.Join(analysistest.TestData(), "src")
	mtlinter.CheckDir(t, filepath.Join(srcRoot, "optin_clean"))
	mtlinter.CheckDir(t, filepath.Join(srcRoot, "optin_ignored"))
}

func TestCheckDirOptinViolation(t *testing.T) {
	srcRoot := filepath.Join(analysistest.TestData(), "src")
	rec := &recordingT{}
	diags := mtlinter.CheckDir(rec, filepath.Join(srcRoot, "optin_violation"))
	// optin_violation has:
	// - 1 forbidden promauto import
	// - 7 forbidden global metric vars (the 3 // mtlint:ignore vars are exempted)
	// - 4 forbidden Register/MustRegister calls (the 2 // mtlint:ignore calls are exempted)
	const wantCount = 12
	if len(diags) != wantCount || len(rec.errors) != wantCount {
		t.Fatalf("CheckDir(optin_violation) reported %d diagnostics (errors=%d), want %d:\n%v", len(diags), len(rec.errors), wantCount, diags)
	}
}

func TestCheckDirWithCheckAndExcludePackages(t *testing.T) {
	srcRoot := filepath.Join(analysistest.TestData(), "src")

	// wildcard_excluded is skipped via WithExcludePackages.
	mtlinter.CheckDir(t, srcRoot,
		mtlinter.WithCheckPackages("wildcard_excluded/..."),
		mtlinter.WithExcludePackages("wildcard_excluded/..."),
	)

	// wildcard_checked fails when explicitly checked via WithCheckPackages.
	rec := &recordingT{}
	diags := mtlinter.CheckDir(rec, srcRoot,
		mtlinter.WithCheckPackages("wildcard_checked/..."),
	)
	if len(diags) == 0 {
		t.Fatalf("CheckDir(wildcard_checked/...) = 0 diagnostics, want > 0")
	}
}

func TestCheckDirInferredPrometheusConstructorTypes(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package controller

import (
	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

var (
	inferredCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "inferred_counter",
	})
	ignoredCounter = prometheus.NewCounter( // mtlint:ignore process-level metric
		prometheus.CounterOpts{
			Name: "ignored_counter",
		},
	)
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "metrics.go"), []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rec := &recordingT{}
	diags := mtlinter.CheckDir(rec, tmpDir)
	if len(diags) != 1 {
		t.Fatalf("CheckDir(%q) = %d diagnostics (%v), want 1 for inferredCounter", tmpDir, len(diags), diags)
	}
}

func TestCheckDirSuppressionValidation(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantSubstrings []string
	}{
		{
			name: "valid_per_spec_and_call_doc_suppressions",
			src: `package controller

import (
	"sync"

	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

var (
	// Preliminary comment before suppression directive.
	// mtlint:ignore Process-level metric not scoped to a single tenant.
	exitCounter = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "exit_total"},
	)
	latency = prometheus.NewHistogramVec( // mtlint:ignore Process-level latency metric.
		prometheus.HistogramOpts{Name: "latency_ms"},
		[]string{"method"},
	)
)

// mtlint:ignore Single unparenthesized var metric.
var buildInfo = prometheus.NewGauge(prometheus.GaugeOpts{Name: "build_info"})

var registerOnce sync.Once

func RegisterMetrics() {
	registerOnce.Do(func() {
		prometheus.MustRegister(exitCounter) // mtlint:ignore Process-level metric.
		// mtlint:ignore Process-level registration with doc comment.
		prometheus.MustRegister(latency, buildInfo)
	})
}
`,
			wantSubstrings: nil,
		},
		{
			name: "bare_and_whitespace_only_ignore_directives",
			src: `package controller

import (
	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

// mtlint:ignore
var bareCounter = prometheus.NewCounter(prometheus.CounterOpts{Name: "bare_counter"})

func init() {
	prometheus.MustRegister(bareCounter) // mtlint:ignore   	
}
`,
			wantSubstrings: []string{
				"metrics.go:8:1: mtlint:ignore requires a reason",
				"metrics.go:12:39: mtlint:ignore requires a reason",
				"metrics.go:9:5: package-level global metric variable is forbidden in MT mode: bareCounter",
				"metrics.go:12:2: direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead",
			},
		},
		{
			name: "var_block_suppression_rejected",
			src: `package controller

import (
	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

// mtlint:ignore Legacy block annotation.
var (
	unannotated = prometheus.NewCounter(prometheus.CounterOpts{Name: "unannotated"})
	// mtlint:ignore Annotated per-spec metric.
	annotated = prometheus.NewCounter(prometheus.CounterOpts{Name: "annotated"})
)
`,
			wantSubstrings: []string{
				"metrics.go:8:1: mtlint:ignore on a var block is not supported; annotate each variable individually",
				"metrics.go:10:2: package-level global metric variable is forbidden in MT mode: unannotated",
			},
		},
		{
			name: "trailing_previous_line_and_inner_multiline_comments_do_not_suppress",
			src: `package controller

import (
	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

var (
	multiLineVar = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "multi_line", // mtlint:ignore Inner line comment.
		},
	) // mtlint:ignore Closing paren comment.
)

func init() {
	x := 1 // mtlint:ignore Trailing comment on line N-1.
	prometheus.MustRegister(multiLineVar)
	_ = x

	prometheus.Register(
		multiLineVar, // mtlint:ignore Inner call argument comment.
	)
}
`,
			wantSubstrings: []string{
				"metrics.go:9:2: package-level global metric variable is forbidden in MT mode: multiLineVar",
				"metrics.go:18:2: direct call to prometheus.MustRegister is forbidden; use mtmetrics factory instead",
				"metrics.go:21:2: direct call to prometheus.Register is forbidden; use mtmetrics factory instead",
			},
		},
		{
			name: "block_and_mid_sentence_comments_ignored",
			src: `package controller

import (
	"google3/third_party/golang/prometheus/client/prometheus/prometheus"
	_ "google3/third_party/tenancy/components/mtmetrics/mtmetrics"
)

/* mtlint:ignore Block comment with reason. */
var blockVar = prometheus.NewCounter(prometheus.CounterOpts{Name: "block_var"})

// Avoid using mtlint:ignore on tenant metrics.
var proseVar = prometheus.NewCounter(prometheus.CounterOpts{Name: "prose_var"})
`,
			wantSubstrings: []string{
				"metrics.go:9:5: package-level global metric variable is forbidden in MT mode: blockVar",
				"metrics.go:12:5: package-level global metric variable is forbidden in MT mode: proseVar",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(tmpDir, "metrics.go"), []byte(tc.src), 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			rec := &recordingT{}
			diags := mtlinter.CheckDir(rec, tmpDir)
			if len(diags) != len(tc.wantSubstrings) {
				t.Fatalf("CheckDir() returned %d diagnostics, want %d:\ngot:  %v\nwant: %v", len(diags), len(tc.wantSubstrings), diags, tc.wantSubstrings)
			}
			for i, wantSub := range tc.wantSubstrings {
				if !strings.Contains(diags[i], wantSub) {
					t.Errorf("diags[%d] = %q, want substring %q", i, diags[i], wantSub)
				}
			}
		})
	}
}
