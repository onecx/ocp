package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	gcrregistry "github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ordinary error", errors.New("not found"), false},
		{"http status", &transport.Error{StatusCode: http.StatusNotFound}, true},
		{"manifest unknown", &transport.Error{Errors: []transport.Diagnostic{{Code: transport.ManifestUnknownErrorCode}}}, true},
		{"not found code", &transport.Error{Errors: []transport.Diagnostic{{Code: "NOT_FOUND"}}}, true},
		{"wrapped error", fmt.Errorf("wrapped: %w", &transport.Error{StatusCode: http.StatusNotFound}), true},
		{"other status", &transport.Error{StatusCode: http.StatusForbidden}, false},
		{"other code", &transport.Error{Errors: []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFound(tt.err); got != tt.want {
				t.Fatalf("isNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func embeddedImageDescriptor(t *testing.T, config []byte) *remote.Descriptor {
	t.Helper()
	digest, size, err := v1.SHA256(bytes.NewReader(config))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.OCIConfigJSON, Digest: digest, Size: size, Data: config,
		},
		Layers: []v1.Descriptor{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &remote.Descriptor{
		Descriptor: v1.Descriptor{MediaType: types.OCIManifestSchema1},
		Manifest:   raw,
	}
}

func TestGetMetadataEmbeddedImage(t *testing.T) {
	desc := embeddedImageDescriptor(t, []byte(`{"architecture":"arm64","os":"linux","variant":"v8","config":{"Env":["JAVA_APP_JAR=app.jar"]}}`))
	platforms, config, err := GetMetadata(desc)
	if err != nil {
		t.Fatal(err)
	}
	if len(platforms) != 1 || !reflect.DeepEqual(platforms[0], &v1.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"}) {
		t.Fatalf("platforms = %#v, want linux/arm64/v8", platforms)
	}
	if !reflect.DeepEqual(config.Config.Env, []string{"JAVA_APP_JAR=app.jar"}) {
		t.Fatalf("config environment = %v", config.Config.Env)
	}
}

func TestGetMetadataErrors(t *testing.T) {
	tests := []struct {
		name string
		desc *remote.Descriptor
	}{
		{"unsupported image", &remote.Descriptor{Descriptor: v1.Descriptor{MediaType: types.DockerManifestSchema1}}},
		{"malformed image manifest", &remote.Descriptor{Descriptor: v1.Descriptor{MediaType: types.OCIManifestSchema1}, Manifest: []byte("{")}},
		{"malformed index manifest", &remote.Descriptor{Descriptor: v1.Descriptor{MediaType: types.OCIImageIndex}, Manifest: []byte("{")}},
		{"malformed image config", embeddedImageDescriptor(t, []byte("not json"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platforms, config, err := GetMetadata(tt.desc)
			if err == nil || platforms != nil || config != nil {
				t.Fatalf("GetMetadata() = %v, %v, %v; want nil metadata and an error", platforms, config, err)
			}
			if got, err := CreateImageDescriptor(tt.desc); err == nil || got != nil {
				t.Fatalf("CreateImageDescriptor() = %v, %v; want nil and an error", got, err)
			}
		})
	}
}

func TestGetMetadataIndexWithoutRunnableImages(t *testing.T) {
	raw, err := json.Marshal(v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests: []v1.Descriptor{
			{Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("0", 64)}},
			{Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("0", 64)}, Platform: &v1.Platform{Architecture: "unknown", OS: "unknown"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	desc := &remote.Descriptor{Descriptor: v1.Descriptor{MediaType: types.OCIImageIndex}, Manifest: raw}
	platforms, config, err := GetMetadata(desc)
	if err != nil || len(platforms) != 0 || config != nil {
		t.Fatalf("GetMetadata() = %v, %v, %v; want no platforms or config", platforms, config, err)
	}
}

func registryServer(t *testing.T, intercept func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	handler := gcrregistry.New(gcrregistry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if intercept != nil && intercept(w, r) {
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func imageTag(t *testing.T, server *httptest.Server, path string) name.Tag {
	t.Helper()
	tag, err := name.NewTag(strings.TrimPrefix(server.URL, "http://") + "/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return tag
}

func configuredImage(t *testing.T, arch string) v1.Image {
	t.Helper()
	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{Architecture: arch, OS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestGetDescriptorsFromRegistry(t *testing.T) {
	var imageRequests atomic.Int64
	server := registryServer(t, func(_ http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && r.URL.Path == "/v2/images/app/manifests/latest" {
			imageRequests.Add(1)
		}
		return false
	})
	amd64 := configuredImage(t, "amd64")
	arm64 := configuredImage(t, "arm64")
	tag := imageTag(t, server, "images/app:latest")
	if err := remote.Write(tag, amd64, remote.WithContext(t.Context())); err != nil {
		t.Fatal(err)
	}
	index := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: amd64, Descriptor: v1.Descriptor{Platform: &v1.Platform{Architecture: "amd64", OS: "linux"}}},
		mutate.IndexAddendum{Add: arm64, Descriptor: v1.Descriptor{Platform: &v1.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"}}},
		mutate.IndexAddendum{Add: empty.Image, Descriptor: v1.Descriptor{Platform: &v1.Platform{Architecture: "unknown", OS: "unknown"}}},
		mutate.IndexAddendum{Add: empty.Image},
	)
	indexTag := imageTag(t, server, "images/multi:latest")
	if err := remote.WriteIndex(indexTag, index, remote.WithContext(t.Context())); err != nil {
		t.Fatal(err)
	}
	missing := imageTag(t, server, "images/missing:latest")
	got, err := GetDescriptors([]string{tag.Name(), tag.Name(), indexTag.Name(), missing.Name()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[missing.Name()] != nil {
		t.Fatalf("descriptors = %v, want only the two existing images", got)
	}
	if n := imageRequests.Load(); n != 1 {
		t.Fatalf("duplicate image fetched %d times, want once", n)
	}
	single := got[tag.Name()]
	if single.Config.Architecture != "amd64" || len(single.Platforms) != 1 {
		t.Fatalf("single-image metadata = %#v", single)
	}
	multi := got[indexTag.Name()]
	wantPlatforms := []*v1.Platform{{Architecture: "amd64", OS: "linux"}, {Architecture: "arm64", OS: "linux", Variant: "v8"}}
	if !reflect.DeepEqual(multi.Platforms, wantPlatforms) || multi.Config.Architecture != "amd64" {
		t.Fatalf("index metadata = %#v, want supported platforms and first image config", multi)
	}
	digest, err := index.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if multi.Descriptor.Digest != digest {
		t.Fatalf("index digest = %s, want %s", multi.Descriptor.Digest, digest)
	}
}

func TestGetDescriptorsEmpty(t *testing.T) {
	got, err := GetDescriptors(nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("GetDescriptors(nil) = %v, %v; want an empty map", got, err)
	}
}

func TestGetDescriptorErrors(t *testing.T) {
	if _, err := GetDescriptor("invalid reference !"); err == nil {
		t.Fatal("invalid image reference accepted")
	}
	if got, err := GetDescriptors([]string{"invalid reference !"}); err == nil || got != nil {
		t.Fatalf("GetDescriptors() = %v, %v; want no partial result and an error", got, err)
	}
	tests := []struct {
		name   string
		status int
		media  types.MediaType
		body   string
	}{
		{"denied", http.StatusForbidden, "application/json", `{"errors":[{"code":"DENIED","message":"denied"}]}`},
		{"unsupported image", http.StatusOK, types.DockerManifestSchema1, `{"schemaVersion":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := registryServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if strings.Contains(r.URL.Path, "/manifests/") {
					w.Header().Set("Content-Type", string(tt.media))
					w.WriteHeader(tt.status)
					_, _ = io.WriteString(w, tt.body)
					return true
				}
				return false
			})
			if got, err := GetDescriptors([]string{imageTag(t, server, "images/broken:latest").Name()}); err == nil || got != nil {
				t.Fatalf("GetDescriptors() = %v, %v; want nil and a propagated error", got, err)
			}
		})
	}
}

func TestGetMetadataIndexDownloadErrors(t *testing.T) {
	for _, failPart := range []string{"/manifests/sha256:", "/blobs/"} {
		t.Run(failPart, func(t *testing.T) {
			var fail atomic.Bool
			server := registryServer(t, func(w http.ResponseWriter, r *http.Request) bool {
				if fail.Load() && r.Method == http.MethodGet && strings.Contains(r.URL.Path, failPart) {
					http.Error(w, "unavailable image metadata", http.StatusForbidden)
					return true
				}
				return false
			})
			index := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
				Add:        configuredImage(t, "amd64"),
				Descriptor: v1.Descriptor{Platform: &v1.Platform{Architecture: "amd64", OS: "linux"}},
			})
			tag := imageTag(t, server, "images/broken:latest")
			if err := remote.WriteIndex(tag, index, remote.WithContext(t.Context())); err != nil {
				t.Fatal(err)
			}
			fail.Store(true)
			desc, err := GetDescriptor(tag.Name())
			if err != nil {
				t.Fatal(err)
			}
			platforms, config, err := GetMetadata(desc)
			if err == nil || platforms != nil || config != nil || !strings.Contains(err.Error(), "403") {
				t.Fatalf("GetMetadata() = %v, %v, %v; want propagated registry error", platforms, config, err)
			}
		})
	}
}
