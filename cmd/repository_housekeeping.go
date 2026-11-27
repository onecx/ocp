package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

type repositoryHousekeepingOptions struct {
	global     *globalOptions
	repository *sharedRepositoryOptions
	product    string
	versions   int
	dryRun     bool
}

func repositoryHousekeepingCmd(global *globalOptions) *cobra.Command {
	sharedRepoFlags, sharedRepoOpts := sharedRepositoryFlags()
	opts := repositoryHousekeepingOptions{
		global:     global,
		repository: sharedRepoOpts,
	}
	cmd := &cobra.Command{
		Use:   "housekeeping",
		Short: "Perform housekeeping tasks for the repository",
		Long: `Delete the manifests of pre-release versions (release candidates) that are no longer needed.

Manifest files are grouped by their base version (X.Y.Z, without the pre-release part). The
pre-release manifests of the latest --versions base versions are kept, the pre-release manifests
of all older base versions are deleted. Final releases are never deleted. Files whose name is
not a semantic version are skipped with a warning.

Example with --versions 2 and the manifests 1.2.0, 1.2.0-rc.1, 1.1.0-rc.2, 1.0.0-rc.1:
the base versions are 1.2.0, 1.1.0 and 1.0.0; only 1.0.0-rc.1 is deleted.

Use --dry-run to only log what would be deleted.`,
		Example: `  # show which release candidates would be deleted
  ocp repository housekeeping --dry-run

  # keep the release candidates of the latest 3 base versions, for one product only
  ocp repository housekeeping --product onecx-test --versions 3

  # clean up another repository directory
  ocp repository housekeeping --repository /srv/ocp`,
		RunE: commandAction(opts.run),
	}
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedRepoFlags)
	flags.StringVar(&opts.product, "product", "", "Specify the product for housekeeping")
	flags.IntVar(&opts.versions, "versions", 2, "Specify the number of latest base versions (X.Y.Z, with or without final release) that keep their release candidates; must be >= 1")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Only log the manifests that would be deleted, do not delete anything")
	return cmd
}

func (opts *repositoryHousekeepingOptions) run(args []string, stdout io.Writer) (retErr error) {
	if opts.versions < 1 {
		return fmt.Errorf("versions must be >= 1, got %d", opts.versions)
	}
	if err := opts.repository.createRepositoryDir(); err != nil {
		return err
	}

	productDirs, err := opts.findProductDirs()
	if err != nil {
		return err
	}
	for _, dir := range productDirs {
		if err := housekeepingProductDir(dir, opts.versions, opts.dryRun); err != nil {
			return err
		}
	}
	return nil
}

// findProductDirs returns all directories (<registry>/<organization>/<product>) containing manifest files.
func (opts *repositoryHousekeepingOptions) findProductDirs() ([]string, error) {
	dirs := map[string]struct{}{}
	err := filepath.WalkDir(opts.repository.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}
		dir := filepath.Dir(path)
		if opts.product != "" && filepath.Base(dir) != opts.product {
			return nil
		}
		dirs[dir] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan repository %s: %w", opts.repository.dir, err)
	}

	result := make([]string, 0, len(dirs))
	for dir := range dirs {
		result = append(result, dir)
	}
	sort.Strings(result)
	return result, nil
}

type housekeepingManifest struct {
	file string
	core string
}

func housekeepingProductDir(dir string, keep int, dryRun bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", dir, err)
	}

	cores := map[string]*semver.Version{}
	var preReleases []housekeepingManifest

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		file := filepath.Join(dir, e.Name())
		v, err := semver.StrictNewVersion(strings.TrimSuffix(e.Name(), ".yaml"))
		if err != nil {
			slog.Warn("skip manifest with non semver name", "file", file)
			continue
		}
		core := semver.New(v.Major(), v.Minor(), v.Patch(), "", "")
		cores[core.String()] = core
		if v.Prerelease() != "" {
			preReleases = append(preReleases, housekeepingManifest{file: file, core: core.String()})
		}
	}

	sorted := make([]*semver.Version, 0, len(cores))
	for _, c := range cores {
		sorted = append(sorted, c)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].GreaterThan(sorted[j]) })

	// the latest N core versions may keep their pre-release (RC) manifests
	protected := map[string]struct{}{}
	for i := 0; i < keep && i < len(sorted); i++ {
		protected[sorted[i].String()] = struct{}{}
	}

	sort.Slice(preReleases, func(i, j int) bool { return preReleases[i].file < preReleases[j].file })
	for _, m := range preReleases {
		if _, ok := protected[m.core]; ok {
			continue
		}
		if dryRun {
			slog.Info("dry-run: would delete manifest", "file", m.file)
			continue
		}
		if err := os.Remove(m.file); err != nil {
			return fmt.Errorf("delete manifest %s: %w", m.file, err)
		}
		slog.Info("deleted manifest", "file", m.file)
	}
	return nil
}
