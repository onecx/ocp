package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	target := filepath.Join(home, ".ocp", "repository")
	productsFile := filepath.Join(root, "products.yaml")
	writeTestFile(t, productsFile, `defaults: {registry: "oci://registry.example", organization: org, product: app}
products:
  - version: 1.0.0
  - version: 2.0.0
  - version: 3.0.0
  - {product: other, version: 1.0.0}
`)
	manifestPath := func(repo, product, version string) string {
		return filepath.Join(repo, "registry.example", "org", product, version+".yaml")
	}
	original := "# preserve formatting\r\nunknown: [unparsed\r\nbinary: \x00"
	writeTestFile(t, manifestPath(source, "app", "1.0.0"), original)
	writeTestFile(t, manifestPath(source, "app", "2.0.0"), "do not overwrite target")
	writeTestFile(t, manifestPath(target, "app", "2.0.0"), "existing target")
	writeTestFile(t, manifestPath(source, "other", "1.0.0"), "another product\n")
	missing := manifestPath(source, "app", "3.0.0")
	wantOutput := fmt.Sprintf("Manifest file does not exist in source repository: %s\n", missing)
	args := []string{"product", "copy", "--products-file", productsFile, "--source-repository", source}

	for attempt := 1; attempt <= 2; attempt++ {
		stdout, err := executeTestCommand(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		if stdout != wantOutput {
			t.Fatalf("attempt %d output = %q, want %q", attempt, stdout, wantOutput)
		}
		for path, want := range map[string]string{
			manifestPath(source, "app", "1.0.0"):   original,
			manifestPath(source, "app", "2.0.0"):   "do not overwrite target",
			manifestPath(target, "app", "1.0.0"):   original,
			manifestPath(target, "app", "2.0.0"):   "existing target",
			manifestPath(target, "other", "1.0.0"): "another product\n",
		} {
			if got := readTestFile(t, path); got != want {
				t.Fatalf("attempt %d: %s = %q, want %q", attempt, path, got, want)
			}
		}
		if _, err := os.Stat(manifestPath(target, "app", "3.0.0")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("missing source created a target manifest: %v", err)
		}
	}
}

func TestProductCopySkipsExistingTargetBeforeReadingSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	productsFile := filepath.Join(root, "products.yaml")
	writeTestFile(t, productsFile, "products: [{registry: registry.example, organization: org, product: app, version: 1}]\n")
	writeTestFile(t, source, "not a directory")
	targetFile := filepath.Join(target, "registry.example", "org", "app", "1.yaml")
	writeTestFile(t, targetFile, "keep")
	stdout, err := executeTestCommand(t, "product", "copy", "--products-file", productsFile,
		"--source-repository", source, "--repository", target)
	if err != nil || stdout != "" {
		t.Fatalf("copy with existing target = %q, %v", stdout, err)
	}
	if got := readTestFile(t, targetFile); got != "keep" {
		t.Fatalf("existing target changed: %q", got)
	}
}

func TestProductCopyErrors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(t *testing.T, opts *productCopyOptions)
		want  string
	}{
		{"missing products file", func(t *testing.T, opts *productCopyOptions) {
			opts.productsFile.file = filepath.Join(t.TempDir(), "missing.yaml")
		}, "read products file"},
		{"invalid products file", func(t *testing.T, opts *productCopyOptions) {
			writeTestFile(t, opts.productsFile.file, "products: [")
		}, "parse products file"},
		{"target stat failure", func(t *testing.T, opts *productCopyOptions) {
			writeTestFile(t, opts.repository.dir, "not a directory")
		}, "check manifest file in repository"},
		{"source stat failure", func(t *testing.T, opts *productCopyOptions) {
			writeTestFile(t, opts.source.dir, "not a directory")
		}, "check manifest file in source repository"},
		{"source read failure", func(t *testing.T, opts *productCopyOptions) {
			path := filepath.Join(opts.source.dir, "registry.example", "org", "app", "1.yaml")
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}, "read source manifest"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			opts := productCopyOptions{
				repository:   &sharedRepositoryOptions{dir: filepath.Join(root, "target")},
				source:       &sharedRepositoryOptions{dir: filepath.Join(root, "source")},
				productsFile: &sharedProductsFileOptions{file: filepath.Join(root, "products.yaml")},
			}
			writeTestFile(t, opts.productsFile.file, "products: [{registry: registry.example, organization: org, product: app, version: 1}]\n")
			tt.setup(t, &opts)
			if err := opts.run(nil, io.Discard); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestProductCopyPropagatesOutputError(t *testing.T) {
	root := t.TempDir()
	productsFile := filepath.Join(root, "products.yaml")
	writeTestFile(t, productsFile, "products: [{registry: registry.example, organization: org, product: app, version: 1}]\n")
	opts := productCopyOptions{
		repository:   &sharedRepositoryOptions{dir: filepath.Join(root, "target")},
		source:       &sharedRepositoryOptions{dir: filepath.Join(root, "source")},
		productsFile: &sharedProductsFileOptions{file: productsFile},
	}
	if err := opts.run(nil, failingWriter{io.ErrClosedPipe}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v, want writer error", err)
	}
}
