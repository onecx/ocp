package template

import (
	"strings"
	"testing"
)

func TestPrint(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{"fields", "{{.Registry}}/{{.Product}}:{{.Version}}", map[string]string{"Registry": "registry.example", "Product": "app", "Version": "1.2.3"}, "registry.example/app:1.2.3"},
		{"sprig functions", `{{ default "fallback" .Name | upper }}:{{ add 2 3 }}`, map[string]string{}, "FALLBACK:5"},
		{"range", "{{range .}}{{.}}\n{{end}}", []string{"first", "second"}, "first\nsecond\n"},
		{"empty template", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Print(tt.template, tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Print() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintErrors(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{"syntax", "{{", nil, "error parse template data"},
		{"unknown function", "{{notAFunction}}", nil, "error parse template data"},
		{"missing struct field", "{{.Missing}}", struct{}{}, "error execute template"},
		{"sprig failure", `{{fail "intentional failure"}}`, nil, "intentional failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Print(tt.template, tt.data)
			if got != "" || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Print() = %q, %v; want empty output and error containing %q", got, err, tt.want)
			}
		})
	}
}
