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

func TestCheckDir(t *testing.T) {
	srcRoot := filepath.Join(analysistest.TestData(), "src")

	t.Run("clean_and_ignored_packages_pass", func(t *testing.T) {
		mtlinter.CheckDir(t, filepath.Join(srcRoot, "optin_clean"))
		mtlinter.CheckDir(t, filepath.Join(srcRoot, "optin_ignored"))
	})

	t.Run("optin_violation_reports_expected_diagnostics", func(t *testing.T) {
		rec := &recordingT{}
		diags := mtlinter.CheckDir(rec, filepath.Join(srcRoot, "optin_violation"))
		// optin_violation has:
		// - 2 forbidden promauto imports
		// - 7 forbidden global metric vars (the 3 // mtlint:ignore vars are exempted)
		// - 4 forbidden Register/MustRegister calls (the 2 // mtlint:ignore calls are exempted)
		const wantCount = 13
		if len(diags) != wantCount || len(rec.errors) != wantCount {
			t.Fatalf("CheckDir reported %d diagnostics (errors=%d), want %d:\n%v", len(diags), len(rec.errors), wantCount, diags)
		}
	})

	t.Run("with_check_and_exclude_packages", func(t *testing.T) {
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
			t.Fatalf("expected violations in wildcard_checked/..., got none")
		}
	})

	t.Run("inferred_prometheus_constructor_types", func(t *testing.T) {
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
			t.Fatalf("expected 1 violation for inferredCounter, got %d: %v", len(diags), diags)
		}
	})
}
