package cmd

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"onecx.tkit.org/ocp/internal/manifest"
)

func TestRepositoryManifestCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	repo := sharedRepositoryOptions{dir: filepath.Join(root, "repository")}
	want := manifest.Manifest{
		ApiVersion: "v1", Organization: "org", Product: "app", Version: "1.0.0",
		Applications: map[string]*manifest.Application{"backend": {Version: "2.0.0"}},
	}
	if err := repo.repositoryWriteManifestFile("oci://registry.example", &want); err != nil {
		t.Fatal(err)
	}
	stdout, err := executeTestCommand(t, "repository", "manifest", "--repository", repo.dir,
		"--registry", "oci://registry.example", "--organization", "org", "--product", "app",
		"--version", "1.0.0", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got manifest.Manifest
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %#v, want %#v", got, want)
	}
}

func TestRepositoryCreateSkipsExistingManifest(t *testing.T) {
	for _, command := range []string{"create", "products"} {
		t.Run(command, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			repository := filepath.Join(root, "repository")
			path := filepath.Join(repository, "registry.example", "org", "app", "1.yaml")
			writeTestFile(t, path, "existing manifest need not be parsed\n")
			args := []string{"repository", command, "--repository", repository, "--chart-url-template", "{{"}
			if command == "create" {
				args = append(args, "--registry", "oci://registry.example", "--organization", "org",
					"--product", "app", "--version", "1")
			} else {
				productsFile := filepath.Join(root, "products.yaml")
				writeTestFile(t, productsFile, "products: [{registry: 'oci://registry.example', organization: org, product: app, version: 1}]\n")
				args = append(args, "--products-file", productsFile)
			}
			stdout, err := executeTestCommand(t, args...)
			if err != nil || stdout != "" {
				t.Fatalf("skip existing manifest = %q, %v", stdout, err)
			}
			if got := readTestFile(t, path); got != "existing manifest need not be parsed\n" {
				t.Fatalf("existing manifest changed: %q", got)
			}
		})
	}
}

func TestProductCommandsRequireFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tt := range []struct {
		command string
		missing []string
	}{
		{"copy", []string{"products-file"}},
		{"report", []string{"products-file", "template"}},
	} {
		t.Run(tt.command, func(t *testing.T) {
			_, err := executeTestCommand(t, "product", tt.command)
			if err == nil || !strings.Contains(err.Error(), "required flag") {
				t.Fatalf("error = %v, want missing required flag error", err)
			}
			for _, flag := range tt.missing {
				if !strings.Contains(err.Error(), `"`+flag+`"`) {
					t.Fatalf("error = %v, want missing flag %q", err, flag)
				}
			}
		})
	}
}
