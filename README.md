# ocp

## Install

```
curl -sSfL "https://github.com/onecx/ocp/releases/download/${OCP_VERSION}/ocp_Linux_x86_64.tar.gz" \
  | sudo tar -xz -C /usr/local/bin ocp
```

## Copy product manifests

```sh
ocp product copy --products-file products.yaml \
  --source-repository /path/to/source --repository /path/to/target
```

Copies the listed products' manifest files without changing their contents,
creating target directories as needed. Existing target manifests and missing
source manifests are skipped.