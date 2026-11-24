# ocp

## Copy product manifests

```sh
ocp product copy --products-file products.yaml \
  --source-repository /path/to/source --repository /path/to/target
```

Copies the listed products' manifest files without changing their contents,
creating target directories as needed. Existing target manifests and missing
source manifests are skipped.