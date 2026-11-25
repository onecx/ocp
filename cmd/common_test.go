package cmd

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"onecx.tkit.org/ocp/internal/manifest"
)

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func executeTestCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	app, _ := createApp(BuildVersion{})
	app.SilenceErrors = true
	app.SilenceUsage = true
	var stdout bytes.Buffer
	app.SetOut(&stdout)
	app.SetErr(io.Discard)
	app.SetArgs(args)
	err := app.Execute()
	return stdout.String(), err
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type stringerValues []string

func (stringerValues) String() string {
	return "custom representation"
}

func TestSharedOutputFormats(t *testing.T) {
	data := struct {
		Name string `json:"name" yaml:"name"`
	}{Name: "app"}
	tests := []struct {
		name   string
		format string
		data   any
		want   string
	}{
		{"json", "json", data, "{\n    \"name\": \"app\"\n}\n"},
		{"yaml", "yaml", data, "name: app\n"},
		{"default yaml", "", data, "name: app\n"},
		{"plain slice", "plain", []string{"first", "second"}, "first\nsecond\n"},
		{"plain array", "plain", [2]int{1, 2}, "1\n2\n"},
		{"plain scalar", "plain", 42, "42\n"},
		{"plain stringer", "plain", stringerValues{"ignored"}, "custom representation\n"},
		{"plain nil", "plain", nil, "<nil>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, destination := range []string{"stdout", "file"} {
				t.Run(destination, func(t *testing.T) {
					opts := sharedOutputOptions{format: tt.format}
					if destination == "file" {
						opts.file = filepath.Join(t.TempDir(), "output")
						writeTestFile(t, opts.file, "old content that must be replaced")
					}
					var stdout bytes.Buffer
					if err := opts.writeOutput(&stdout, tt.data); err != nil {
						t.Fatal(err)
					}
					got := stdout.String()
					if destination == "file" {
						if stdout.Len() != 0 {
							t.Fatalf("file output also wrote to stdout: %q", stdout.String())
						}
						got = readTestFile(t, opts.file)
					}
					if got != tt.want {
						t.Fatalf("output = %q, want %q", got, tt.want)
					}
				})
			}
		})
	}
}

func TestSharedOutputNormalizesTrailingNewlines(t *testing.T) {
	for _, data := range []string{"report", "report\n", "report\r\n\n", ""} {
		t.Run(data, func(t *testing.T) {
			var stdout bytes.Buffer
			opts := sharedOutputOptions{}
			if err := opts.write([]byte(data), &stdout); err != nil {
				t.Fatal(err)
			}
			want := "report\n"
			if data == "" {
				want = "\n"
			}
			if got := stdout.String(); got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestSharedOutputErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		format string
		data   any
		want   string
	}{
		{"unsupported format", "xml", "data", "unsupported format: xml"},
		{"invalid json value", "json", make(chan int), "unsupported type"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			opts := sharedOutputOptions{format: tt.format}
			if err := opts.writeOutput(&stdout, tt.data); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("wrote output on failure: %q", stdout.String())
			}
		})
	}
	t.Run("stdout failure", func(t *testing.T) {
		opts := sharedOutputOptions{format: "plain"}
		if err := opts.writeOutput(failingWriter{io.ErrClosedPipe}, "data"); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("error = %v, want writer error", err)
		}
	})
	t.Run("file failure", func(t *testing.T) {
		opts := sharedOutputOptions{format: "plain", file: filepath.Join(t.TempDir(), "missing", "output")}
		var stdout bytes.Buffer
		if err := opts.writeOutput(&stdout, "data"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("error = %v, want file error", err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("file failure fell back to stdout: %q", stdout.String())
		}
	})
}

func TestRepositoryManifestRoundTrip(t *testing.T) {
	repo := sharedRepositoryOptions{dir: filepath.Join(t.TempDir(), "repository")}
	want := &manifest.Manifest{
		ApiVersion: "v1", Organization: "org", Product: "app", Version: "1.0.0+build",
		Helm: "oci://registry.example/org/charts/app:1.0.0_build", Digest: "sha256:chart",
		Applications: map[string]*manifest.Application{
			"backend": {Version: "2.0.0", Images: []*manifest.Image{{Image: "registry.example/backend:2", Arch: []string{"amd64", "arm64"}}}},
		},
	}
	if err := repo.repositoryWriteManifestFile("oci://registry.example", want); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.dir, "registry.example", "org", "app", "1.0.0+build.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest not written to expected repository path: %v", err)
	}
	got, err := repositoryLoadManifest(&repo, "registry.example", "org", "app", "1.0.0+build")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest = %#v, want %#v", got, want)
	}
}

func TestCreateRepositoryDir(t *testing.T) {
	t.Run("creates nested directory and preserves existing contents", func(t *testing.T) {
		repo := sharedRepositoryOptions{dir: filepath.Join(t.TempDir(), "nested", "repository")}
		if err := repo.createRepositoryDir(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(repo.dir, "existing")
		writeTestFile(t, path, "keep")
		if err := repo.createRepositoryDir(); err != nil {
			t.Fatal(err)
		}
		if got := readTestFile(t, path); got != "keep" {
			t.Fatalf("existing content changed: %q", got)
		}
	})
	t.Run("rejects a regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "repository")
		writeTestFile(t, path, "keep")
		repo := sharedRepositoryOptions{dir: path}
		if err := repo.createRepositoryDir(); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("error = %v, want non-directory error", err)
		}
		if got := readTestFile(t, path); got != "keep" {
			t.Fatalf("existing file changed: %q", got)
		}
	})
}
