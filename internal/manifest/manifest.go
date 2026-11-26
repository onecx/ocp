package manifest

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"onecx.tkit.org/ocp/internal/helm"
	"onecx.tkit.org/ocp/internal/registry"
)

const (
	Registry       = "oci://ghcr.io"
	ChartURL       = Registry + "/%s/charts/%s:%s"
	ImageURL       = Registry + "/%s/%s:%s"
	NativeImageURL = ImageURL + "-native"
)

type Manifest struct {
	ApiVersion   string                  `json:"apiVersion" yaml:"apiVersion"`
	Organization string                  `json:"organization" yaml:"organization"`
	Product      string                  `json:"product" yaml:"product"`
	Version      string                  `json:"version" yaml:"version"`
	CreatedAt    string                  `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	Revision     string                  `json:"revision,omitempty" yaml:"revision,omitempty"`
	BuildVersion string                  `json:"buildVersion,omitempty" yaml:"buildVersion,omitempty"`
	Helm         string                  `json:"helm,omitempty" yaml:"helm,omitempty"`
	Digest       string                  `json:"digest,omitempty" yaml:"digest,omitempty"`
	Applications map[string]*Application `json:"applications,omitempty" yaml:"applications,omitempty"`
}

type Application struct {
	Helm    string   `json:"helm,omitempty" yaml:"helm,omitempty"`
	Version string   `json:"version,omitempty" yaml:"version,omitempty"`
	Digest  string   `json:"digest,omitempty" yaml:"digest,omitempty"`
	Images  []*Image `json:"images,omitempty" yaml:"images,omitempty"`
}

type Image struct {
	Image        string   `json:"image,omitempty" yaml:"image,omitempty"`
	Digest       string   `json:"digest,omitempty" yaml:"digest,omitempty"`
	Type         string   `json:"type,omitempty" yaml:"type,omitempty"`
	Kind         string   `json:"kind,omitempty" yaml:"kind,omitempty"`
	Arch         []string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	Key          string   `json:"key,omitempty" yaml:"key,omitempty"`
	CreatedAt    string   `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	Revision     string   `json:"revision,omitempty" yaml:"revision,omitempty"`
	BuildVersion string   `json:"buildVersion,omitempty" yaml:"buildVersion,omitempty"`
	Source       string   `json:"source,omitempty" yaml:"source,omitempty"`
}

type Config struct {
	Organization string   `json:"organization" yaml:"organization"`
	Product      string   `json:"product" yaml:"product"`
	Version      string   `json:"version" yaml:"version"`
	ExcludeApps  []string `json:"excludeApps" yaml:"excludeApps"`
	Url          string   `json:"url" yaml:"url"`
	HelmValues   []string `json:"helmValues" yaml:"helmValues"`
}

func LoadManifestFromFile(file string) (*Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", file, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", file, err)
	}
	return &manifest, nil
}

func CreateManifest(cfg *Config) (*Manifest, error) {

	chrt, url, err := helm.GetChart(cfg.Url)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	template, err := helm.TemplateImagesFromChart(chrt, cfg.Product, cfg.HelmValues)
	if err != nil {
		return nil, fmt.Errorf("template images from chart: %w", err)
	}

	items := []string{}
	for _, item := range template.Images {
		for _, i := range item {
			items = append(items, i.Image)
		}
	}

	descriptors, err := registry.GetDescriptors(items)
	if err != nil {
		return nil, fmt.Errorf("get image descriptors: %w", err)
	}

	apps := map[string]*Application{}
	for _, dep := range chrt.Lock.Dependencies {
		appName := dep.Name
		if slices.Contains(cfg.ExcludeApps, appName) {
			continue
		}

		images := []*Image{}
		if tmp := template.Images[appName]; tmp != nil {
			for _, img := range tmp {

				item := &Image{
					Image: img.Image,
					Kind:  img.Kind,
					Key:   img.Key,
				}

				desc := descriptors[img.Image]
				if desc == nil {
					continue
				}
				item.Arch = getArchs(desc)
				item.Type = string(detectType(desc))
				item.Digest = desc.Descriptor.Digest.String()

				a := desc.Descriptor.Annotations
				if a != nil {
					item.Source = a["org.opencontainers.image.source"]
					item.CreatedAt = value(a, "samo.project.created", "org.opencontainers.image.created")
					item.Revision = value(a, "samo.project.revision", "org.opencontainers.image.revision")
					item.BuildVersion = value(a, "samo.project.version", "org.opencontainers.image.version")
				}
				images = append(images, item)
			}
		}

		helmUrl := fmt.Sprintf("%s/%s:%s", strings.TrimRight(dep.Repository, "/"), dep.Name, strings.ReplaceAll(dep.Version, "+", "_"))
		hd, err := helm.DigestChart(helmUrl)
		if err != nil {
			return nil, fmt.Errorf("digest chart %s: %w", helmUrl, err)
		}
		app := &Application{
			Version: dep.Version,
			Helm:    helmUrl,
			Digest:  hd,
			Images:  images,
		}
		apps[appName] = app
	}

	createdAt := ""
	buildVersion := ""
	revision := ""

	if chrt.Metadata != nil {
		a := chrt.Metadata.Annotations
		if a != nil {
			buildVersion = a["samo.project.version"]
			revision = a["samo.project.revision"]
			createdAt = a["samo.project.created"]
		}
	}

	hd, err := helm.DigestChart(url)
	if err != nil {
		return nil, fmt.Errorf("digest chart %s: %w", url, err)
	}
	return &Manifest{
		ApiVersion:   "v1",
		Organization: cfg.Organization,
		Product:      cfg.Product,
		Version:      cfg.Version,
		Helm:         url,
		Digest:       hd,
		Applications: apps,
		CreatedAt:    createdAt,
		BuildVersion: buildVersion,
		Revision:     revision,
	}, nil
}

type ImageType string

const (
	TypeUnknown ImageType = "unknown"
	TypeJVM     ImageType = "jvm"
	TypeNative  ImageType = "native"
)

func getArchs(desc *registry.ImageDescriptor) []string {
	archs := []string{}
	if desc == nil {
		return archs
	}
	for _, p := range desc.Platforms {
		a := p.Architecture
		if p.Variant != "" {
			a += "/" + p.Variant
		}
		archs = append(archs, a)
	}
	return archs
}

func detectType(desc *registry.ImageDescriptor) ImageType {
	if desc == nil || desc.Config == nil {
		return TypeUnknown
	}
	cfg := desc.Config
	for _, e := range cfg.Config.Env {
		if strings.HasPrefix(e, "JAVA_APP_JAR=") {
			return TypeJVM
		}
	}
	for _, s := range cfg.Config.Entrypoint {
		if strings.Contains(s, "run-java.sh") {
			return TypeJVM
		}
	}
	return TypeNative
}

func value(m map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	return ""
}
