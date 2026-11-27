# ocp

`ocp` builds and maintains a local repository of OneCX product manifests. A manifest describes one
product version: its applications and the container images they use (digest, type, architectures,
labels). Manifests are created from the product's Helm chart, can be copied between repositories
and are the input for reports.

## Install

```
curl -sSfL "https://github.com/onecx/ocp/releases/download/${OCP_VERSION}/ocp_Linux_x86_64.tar.gz" \
  | sudo tar -xz -C /usr/local/bin ocp
```

## Quick start

```sh
# create the manifest of one product version
ocp repository create --registry oci://ghcr.io --organization onecx \
  --product onecx-test --version 1.14.0

# show it
ocp repository manifest --registry oci://ghcr.io --organization onecx \
  --product onecx-test --version 1.14.0
```

Every command has its own help with a description and examples: `ocp <command> --help`.
Add `--debug` to any command for debug logging.

## Repository

Manifests are stored as files, by default in `~/.ocp/repository` (flag `--repository`, `-d`):

```
<repository>/<registry without oci://>/<organization>/<product>/<version>.yaml
e.g. ~/.ocp/repository/ghcr.io/onecx/onecx-test/1.14.0.yaml
```

Creating manifests is incremental: a manifest that already exists is never downloaded again.

### Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repository`, `-d` | `~/.ocp/repository` | Repository directory |
| `--repository-url-template` | `{{.Registry}}/{{.Organization}}/charts/{{.Product}}` | Where the chart versions are listed. Variables: `Registry`, `Organization`, `Product` |
| `--chart-url-template` | `{{.Registry}}/{{.Organization}}/charts/{{.Product}}:{{.Version}}` | Where a chart is downloaded. Variables: `Registry`, `Organization`, `Product`, `Version` |
| `--skip-dependency` | `helm-product` | Comma-separated chart dependencies left out of the manifest |
| `--helm-value` | | Comma-separated Helm values used to render the chart, e.g. `a.b=c,d=e` |

### Input files

Both file formats support `defaults` that every product inherits and reject duplicate products.

Products file (`--products-file`, used by `repository products`, `product copy`, `product report`):

```yaml
defaults:
  registry: oci://ghcr.io
  organization: onecx
products:
  - product: onecx-test
    version: 1.14.0
  - product: onecx-test
    version: 1.13.0
    organization: other-org   # overrides the default
```

Create file (`--file`, used by `repository creates`). It lists products without a version, `count`
is the number of latest versions to create (default 10):

```yaml
defaults:
  registry: oci://ghcr.io
  organization: onecx
  count: 5
products:
  - product: onecx-test
  - product: onecx-workspace
    count: 2
```

## Commands

### `ocp repository create`

Creates the manifest of one product version. `--registry`, `--organization`, `--product` and
`--version` are required.

```sh
ocp repository create --registry oci://ghcr.io --organization onecx \
  --product onecx-test --version 1.14.0

# with extra Helm values and another repository directory
ocp repository create --registry oci://ghcr.io --organization onecx \
  --product onecx-test --version 1.14.0 --repository /srv/ocp --helm-value global.foo=bar
```

### `ocp repository versions`

Creates the manifests of the latest `--latest` versions (default 20) of one product, in the order
the registry returns them.

```sh
ocp repository versions --registry oci://ghcr.io --organization onecx --product onecx-test --latest 5
```

### `ocp repository products`

Creates the manifests of all product versions in a products file.

```sh
ocp repository products --products-file products.yaml
```

### `ocp repository creates`

Creates the manifests of the latest versions of every product in a create file.

```sh
ocp repository creates --file create.yaml
```

### `ocp repository manifest`

Prints a manifest from the repository as `yaml` (default), `json` or `plain`
(`--format`, `-f`), to stdout or to a file (`--output`, `-o`).

```sh
ocp repository manifest --registry oci://ghcr.io --organization onecx \
  --product onecx-test --version 1.14.0 -f json -o manifest.json
```

### `ocp repository housekeeping`

Deletes manifests of pre-release versions (release candidates) that are no longer needed. The
pre-release manifests of the latest `--versions` base versions (`X.Y.Z`, default 2) are kept, the
ones of older base versions are deleted. Final releases are never deleted. `--product` limits the
cleanup to one product, `--dry-run` only logs what would be deleted.

With `--versions 2` and the manifests `1.2.0`, `1.2.0-rc.1`, `1.1.0-rc.2`, `1.0.0-rc.1`, only
`1.0.0-rc.1` is deleted.

```sh
ocp repository housekeeping --dry-run
ocp repository housekeeping --product onecx-test --versions 3
```

### `ocp product copy`

Copies the manifests of the products in a products file from `--source-repository` to
`--repository` without changing them. Existing target manifests and manifests missing in the
source are skipped.

```sh
ocp product copy --products-file products.yaml \
  --source-repository /path/to/source --repository /path/to/target
```

### `ocp product report`

Renders a [Go template](https://pkg.go.dev/text/template) with the manifests of the products in a
products file. The manifests must exist in the repository. The result goes to stdout or to
`--output`, the template defines the format (`--format` is not used).

```sh
ocp product report --products-file products.yaml --template report.tpl -o report.txt
```

The template receives `.Manifests`, the list of manifests. Each manifest has `Organization`,
`Product`, `Version`, `Digest`, `Helm` and `Applications`, a map from the application name to its
`Images`. An image has `Image`, `Digest`, `Type` (`jvm` or `native`), `Kind`, `Arch`, `Key`,
`CreatedAt`, `Revision`, `BuildVersion` and `Source`.

Template functions: all [sprig](https://masterminds.github.io/sprig/) functions and `prefer`.

#### `prefer <images> <attribute> <value>...`

Filters a list of images so that every image available in several variants appears once. The order
of the values is the priority: the variant whose attribute value comes first wins.

An image with the tag suffix `-native` is linked with the image of the same reference without the
suffix, e.g. `onecx-test-oidc:1.8.0-native` (`Type: native`) and `onecx-test-oidc:1.8.0`
(`Type: jvm`). Images without a counterpart are kept, as are images whose attribute value is not in
the list. The order of the images is preserved. The attribute is the Go field name of the image
(`Type`, `Kind`, ...); an unknown name fails the report.

```
{{- range .Manifests }}
{{ .Product }} {{ .Version }}
{{- range $name, $app := .Applications }}
  {{ $name }}
{{- range prefer $app.Images "Type" "native" "jvm" }}
    {{ .Image }}@{{ .Digest }}
{{- end }}
{{- end }}
{{- end }}
```

With the native image available this prints `...oidc:1.8.0-native@sha256:...`, otherwise the jvm
image. Use `prefer $app.Images "Type" "jvm" "native"` to prefer jvm instead.

### `ocp version`

Prints the build version, commit and date.

## Development

```sh
go build ./...
go test ./...
```
