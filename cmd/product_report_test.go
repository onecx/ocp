package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductReport(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	repository := filepath.Join(root, "repository")
	productsFile := filepath.Join(root, "products.yaml")
	templateFile := filepath.Join(root, "report.tmpl")
	writeTestFile(t, productsFile, `defaults: {registry: "oci://registry.example", organization: org, product: app}
products:
  - version: 2.0.0
  - version: 1.0.0
`)
	for _, version := range []string{"1.0.0", "2.0.0"} {
		path := filepath.Join(repository, "registry.example", "org", "app", version+".yaml")
		writeTestFile(t, path, "apiVersion: v1\norganization: org\nproduct: app\nversion: "+version+"\n")
	}
	writeTestFile(t, templateFile, "{{range .Manifests}}{{.Product | upper}}:{{.Version}}\n{{end}}\n\n")
	want := "APP:2.0.0\nAPP:1.0.0\n"
	for _, destination := range []string{"stdout", "file"} {
		t.Run(destination, func(t *testing.T) {
			args := []string{"product", "report", "--products-file", productsFile,
				"--repository", repository, "--template", templateFile}
			outputFile := filepath.Join(root, "report.txt")
			if destination == "file" {
				args = append(args, "--output", outputFile)
			}
			stdout, err := executeTestCommand(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			got := stdout
			if destination == "file" {
				if stdout != "" {
					t.Fatalf("file report also wrote to stdout: %q", stdout)
				}
				got = readTestFile(t, outputFile)
			}
			if got != want {
				t.Fatalf("report = %q, want %q", got, want)
			}
		})
	}
}

func TestProductReportErrors(t *testing.T) {
	for _, tt := range []struct {
		name     string
		products string
		manifest string
		template string
		want     string
	}{
		{"invalid products", "products: [", "", "", "parse products file"},
		{"missing manifest", "products: [{registry: registry.example, organization: org, product: app, version: 1}]", "", "", "manifest file does not exist"},
		{"invalid manifest", "products: [{registry: registry.example, organization: org, product: app, version: 1}]", "applications: [", "", "load manifest from file"},
		{"missing template", "products: []", "", "", "report.tmpl"},
		{"invalid template", "products: []", "", "{{", "error parse template data"},
		{"template execution", "products: []", "", "{{.Missing}}", "error execute template"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			productsFile := filepath.Join(root, "products.yaml")
			repository := filepath.Join(root, "repository")
			templateFile := filepath.Join(root, "report.tmpl")
			writeTestFile(t, productsFile, tt.products)
			if tt.manifest != "" {
				writeTestFile(t, filepath.Join(repository, "registry.example", "org", "app", "1.yaml"), tt.manifest)
			}
			if tt.template != "" {
				writeTestFile(t, templateFile, tt.template)
			}
			stdout, err := executeTestCommand(t, "product", "report", "--products-file", productsFile,
				"--repository", repository, "--template", templateFile)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout != "" {
				t.Fatalf("report wrote partial output on failure: %q", stdout)
			}
		})
	}
}
