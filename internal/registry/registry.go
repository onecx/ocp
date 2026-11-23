package registry

import (
	"errors"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type ImageDescriptor struct {
	Descriptor *remote.Descriptor
	Platforms  []*v1.Platform
	Config     *v1.ConfigFile
}

func GetDescriptors(urls []string) (map[string]*ImageDescriptor, error) {

	tmp := map[string]*ImageDescriptor{}

	for _, url := range urls {
		if _, e := tmp[url]; e {
			continue
		}
		desc, err := GetDescriptor(url)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, err
		}
		item, err := CreateImageDescriptor(desc)
		if err != nil {
			return nil, err
		}
		tmp[url] = item
	}
	return tmp, nil
}

func isNotFound(err error) bool {
	var terr *transport.Error
	if !errors.As(err, &terr) {
		return false
	}
	if terr.StatusCode == http.StatusNotFound {
		return true
	}
	for _, d := range terr.Errors {
		if d.Code == transport.ManifestUnknownErrorCode || d.Code == "NOT_FOUND" {
			return true
		}
	}
	return false
}

func CreateImageDescriptor(desc *remote.Descriptor) (*ImageDescriptor, error) {
	platforms, cfg, err := GetMetadata(desc)
	if err != nil {
		return nil, err
	}
	return &ImageDescriptor{Descriptor: desc, Platforms: platforms, Config: cfg}, nil
}

func GetMetadata(desc *remote.Descriptor) ([]*v1.Platform, *v1.ConfigFile, error) {
	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, nil, err
		}
		im, err := idx.IndexManifest()
		if err != nil {
			return nil, nil, err
		}
		var res []*v1.Platform
		var first *v1.Hash
		var cfg *v1.ConfigFile
		for _, m := range im.Manifests {
			// Skip attestation/SBOM entries ("unknown/unknown").
			if m.Platform == nil || m.Platform.Architecture == "unknown" {
				continue
			}
			if first == nil {
				d := m.Digest
				first = &d
			}
			res = append(res, m.Platform)
		}

		if first != nil {
			img, err := idx.Image(*first)
			if err != nil {
				return nil, nil, err
			}
			cfg, err = img.ConfigFile()
			if err != nil {
				return nil, nil, err
			}
		}
		return res, cfg, nil
	}

	img, err := desc.Image()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, nil, err
	}
	return []*v1.Platform{cfg.Platform()}, cfg, nil
}
func GetDescriptor(url string) (*remote.Descriptor, error) {
	ref, err := name.ParseReference(url)
	if err != nil {
		return nil, err
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}
	return desc, nil
}
