package mtlinter_test

import (
	"fmt"
	"os"
	"path/filepath"
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
