package mtlinter_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtlinter"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestLinter(t *testing.T) {
	testdata := analysistest.TestData()

	resetFlags := func() {
		if err := mtlinter.Analyzer.Flags.Set("check-packages", ""); err != nil {
			t.Fatalf("failed to reset check-packages: %v", err)
		}
		if err := mtlinter.Analyzer.Flags.Set("exclude-packages", ""); err != nil {
			t.Fatalf("failed to reset exclude-packages: %v", err)
		}
	}

	// 1. Default opt-in behavior.
	t.Run("default_opt_in", func(t *testing.T) {
		resetFlags()
		// optin_violation: imports mtmetrics -> checked, fails.
		// optin_clean: imports mtmetrics -> checked, passes.
		// optin_ignored: no import -> not checked, passes (despite violations).
		analysistest.Run(t, testdata, mtlinter.Analyzer, "optin_violation", "optin_clean", "optin_ignored")
	})

	// 2. Explicitly check package via flag.
	t.Run("check-packages", func(t *testing.T) {
		resetFlags()
		if err := mtlinter.Analyzer.Flags.Set("check-packages", "flag_checked"); err != nil {
			t.Fatalf("failed to set check-packages: %v", err)
		}
		// flag_checked: no import -> checked via flag, fails.
		analysistest.Run(t, testdata, mtlinter.Analyzer, "flag_checked")
	})

	// 3. Exclude package via flag.
	t.Run("exclude-packages", func(t *testing.T) {
		resetFlags()
		if err := mtlinter.Analyzer.Flags.Set("exclude-packages", "flag_excluded"); err != nil {
			t.Fatalf("failed to set exclude-packages: %v", err)
		}
		// flag_excluded: imports mtmetrics -> would check, but excluded via flag, passes.
		analysistest.Run(t, testdata, mtlinter.Analyzer, "flag_excluded")
	})

	// 4. Wildcard check-packages and exclude-packages.
	t.Run("wildcard", func(t *testing.T) {
		resetFlags()
		if err := mtlinter.Analyzer.Flags.Set("check-packages", "wildcard_checked/... "); err != nil {
			t.Fatalf("failed to set check-packages: %v", err)
		}
		if err := mtlinter.Analyzer.Flags.Set("exclude-packages", " wildcard_excluded/... "); err != nil {
			t.Fatalf("failed to set exclude-packages: %v", err)
		}
		// wildcard_checked/pkg1: no import -> checked via wildcard, fails.
		// wildcard_excluded/pkg2: imports mtmetrics -> excluded via wildcard, passes.
		analysistest.Run(t, testdata, mtlinter.Analyzer, "wildcard_checked/pkg1", "wildcard_excluded/pkg2")
	})
}
