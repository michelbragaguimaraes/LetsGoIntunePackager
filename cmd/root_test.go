package cmd

import (
	"strings"
	"testing"
)

func TestSetVersionInfoReachesCobra(t *testing.T) {
	// rootCmd is built during package initialization, before main can supply
	// the values baked in by -ldflags. SetVersionInfo has to push them into
	// cobra or --version reports "dev" from a tagged build.
	t.Cleanup(func() { SetVersionInfo("dev", "unknown") })

	SetVersionInfo("v1.2.3", "2026-09-21_12:00:00")

	if rootCmd.Version != "v1.2.3" {
		t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, "v1.2.3")
	}

	tmpl := rootCmd.VersionTemplate()
	if !strings.Contains(tmpl, "v1.2.3") {
		t.Errorf("version template %q does not mention the version", tmpl)
	}
	if !strings.Contains(tmpl, "2026-09-21_12:00:00") {
		t.Errorf("version template %q does not mention the build time", tmpl)
	}
}

func TestVersionTemplateEndsWithNewline(t *testing.T) {
	got := versionTemplate("v1.0.0", "now")
	want := "LetsGoIntunePackager version v1.0.0 (built now)\n"
	if got != want {
		t.Errorf("versionTemplate() = %q, want %q", got, want)
	}
}
