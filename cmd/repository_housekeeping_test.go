package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func listYamlVersions(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var result []string
	for _, e := range entries {
		result = append(result, e.Name())
	}
	sort.Strings(result)
	return result
}

func TestRepositoryHousekeeping(t *testing.T) {
	repo := t.TempDir()
	shell := filepath.Join(repo, "ghcr.io", "onecx", "onecx-shell")
	other := filepath.Join(repo, "ghcr.io", "onecx", "onecx-other")
	for _, v := range []string{"1.3.0-rc1", "1.2.0", "1.2.0-rc1", "1.1.0", "1.1.0-rc2", "1.0.0", "1.0.0-rc1", "invalid"} {
		writeTestFile(t, filepath.Join(shell, v+".yaml"), "")
	}
	writeTestFile(t, filepath.Join(other, "1.0.0-rc1.yaml"), "")
	writeTestFile(t, filepath.Join(other, "0.9.0-rc1.yaml"), "")

	_, err := executeTestCommand(t, "repository", "housekeeping", "-d", repo, "--product", "onecx-shell", "--versions", "2")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"1.0.0.yaml", "1.1.0.yaml", "1.2.0-rc1.yaml", "1.2.0.yaml", "1.3.0-rc1.yaml", "invalid.yaml"}
	if got := listYamlVersions(t, shell); !equalStrings(got, want) {
		t.Fatalf("onecx-shell files = %v, want %v", got, want)
	}
	if got := listYamlVersions(t, other); len(got) != 2 {
		t.Fatalf("other product must not be touched, got %v", got)
	}
	if _, err := executeTestCommand(t, "repository", "housekeeping", "-d", repo, "--versions", "1"); err != nil {
		t.Fatal(err)
	}
	if got, want := listYamlVersions(t, other), []string{"1.0.0-rc1.yaml"}; !equalStrings(got, want) {
		t.Fatalf("onecx-other files = %v, want %v", got, want)
	}
}

func TestRepositoryHousekeepingNegativeVersions(t *testing.T) {
	for _, v := range []string{"-1", "0"} {
		if _, err := executeTestCommand(t, "repository", "housekeeping", "-d", t.TempDir(), "--versions", v); err == nil {
			t.Fatalf("expected error for versions %s", v)
		}
	}
}

func TestRepositoryHousekeepingDefaultVersions(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "ghcr.io", "onecx", "onecx-shell")
	for _, v := range []string{"1.2.0-rc1", "1.1.0-rc1", "1.0.0-rc1", "1.0.0"} {
		writeTestFile(t, filepath.Join(dir, v+".yaml"), "")
	}
	if _, err := executeTestCommand(t, "repository", "housekeeping", "-d", repo); err != nil {
		t.Fatal(err)
	}
	want := []string{"1.0.0.yaml", "1.1.0-rc1.yaml", "1.2.0-rc1.yaml"}
	if got := listYamlVersions(t, dir); !equalStrings(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRepositoryHousekeepingDryRun(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "ghcr.io", "onecx", "onecx-shell")
	want := []string{"1.0.0-rc1.yaml", "1.0.0.yaml", "1.1.0.yaml", "1.2.0.yaml"}
	for _, v := range want {
		writeTestFile(t, filepath.Join(dir, v), "")
	}
	if _, err := executeTestCommand(t, "repository", "housekeeping", "-d", repo, "--versions", "1", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if got := listYamlVersions(t, dir); !equalStrings(got, want) {
		t.Fatalf("dry-run must not delete, files = %v, want %v", got, want)
	}
}
