package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/salaboy/skills-oci/pkg/skill"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

// PullOptions configures a pull operation.
type PullOptions struct {
	Reference            string   // Full OCI reference, e.g., "ghcr.io/org/skills/my-skill:1.0.0"
	OutputDir            string   // Primary directory to extract skill into
	AdditionalOutputDirs []string // Extra directories to also extract the skill into
	PlainHTTP            bool

	OnStatus func(phase string)
}

// PullResult is returned after a successful pull.
type PullResult struct {
	Name       string
	Version    string
	Digest     string
	ExtractTo  string
	Registry   string // e.g., "ghcr.io"
	Repository string // e.g., "org/skills/my-skill"
	Tag        string // e.g., "1.0.0"
}

// FullRef returns the fully-qualified digest-pinned OCI reference.
func (r *PullResult) FullRef() string {
	return fmt.Sprintf("%s/%s:%s@%s", r.Registry, r.Repository, r.Tag, r.Digest)
}

// Source returns the OCI repository reference without tag or digest (for skills.json).
func (r *PullResult) Source() string {
	return fmt.Sprintf("%s/%s", r.Registry, r.Repository)
}

// Pull fetches a skill artifact from a remote registry and extracts it.
func Pull(ctx context.Context, opts PullOptions) (*PullResult, error) {
	if opts.OnStatus != nil {
		opts.OnStatus("Resolving reference")
	}

	ref := opts.Reference
	registry, repository, tag := parseReference(ref)

	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("creating remote repository: %w", err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	repo.Client = NewAuthClient()

	if opts.OnStatus != nil {
		opts.OnStatus("Pulling artifact")
	}

	// Pull to memory store
	store := memory.New()
	desc, err := oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pulling from registry: %w", err)
	}

	if opts.OnStatus != nil {
		opts.OnStatus("Reading manifest")
	}

	// Fetch the manifest to find config and layers
	manifestReader, err := store.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer manifestReader.Close()

	manifestData, err := io.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Read config to get skill metadata
	configReader, err := store.Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("fetching config: %w", err)
	}
	defer configReader.Close()

	configData, err := io.ReadAll(configReader)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var skillConfig skill.SkillConfig
	if err := json.Unmarshal(configData, &skillConfig); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if opts.OnStatus != nil {
		opts.OnStatus("Extracting skill")
	}

	// Find and extract the content layer
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(".", ".agents", "skills")
	}

	for _, layer := range manifest.Layers {
		if layer.MediaType == ContentMediaType {
			layerReader, err := store.Fetch(ctx, layer)
			if err != nil {
				return nil, fmt.Errorf("fetching content layer: %w", err)
			}
			defer layerReader.Close()

			if err := extractTarGz(layerReader, outputDir); err != nil {
				return nil, fmt.Errorf("extracting content: %w", err)
			}
			break
		}
	}

	extractPath := filepath.Join(outputDir, skillConfig.Name)

	// For each additional output directory, create a symlink (Unix/macOS) or
	// copy the files (Windows) pointing to the primary extracted directory.
	for _, additionalDir := range opts.AdditionalOutputDirs {
		if err := os.MkdirAll(additionalDir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", additionalDir, err)
		}
		additionalSkillDir := filepath.Join(additionalDir, skillConfig.Name)

		if runtime.GOOS == "windows" {
			if err := copyDir(extractPath, additionalSkillDir); err != nil {
				return nil, fmt.Errorf("copying skill to %s: %w", additionalSkillDir, err)
			}
		} else {
			// Compute a relative symlink target so the project stays portable.
			relTarget, err := filepath.Rel(additionalDir, extractPath)
			if err != nil {
				return nil, fmt.Errorf("computing relative path from %s to %s: %w", additionalDir, extractPath, err)
			}
			// Remove any stale symlink or directory before creating the new one.
			_ = os.Remove(additionalSkillDir)
			if err := os.Symlink(relTarget, additionalSkillDir); err != nil {
				if !errors.Is(err, os.ErrExist) {
					return nil, fmt.Errorf("creating symlink %s -> %s: %w", additionalSkillDir, relTarget, err)
				}
				// Symlink already exists — check if it already points to the right target.
				existing, readErr := os.Readlink(additionalSkillDir)
				if readErr != nil || existing != relTarget {
					return nil, fmt.Errorf("creating symlink %s -> %s: %w", additionalSkillDir, relTarget, err)
				}
				// Already correct, nothing to do.
			}
		}
	}

	return &PullResult{
		Name:       skillConfig.Name,
		Version:    skillConfig.Version,
		Digest:     desc.Digest.String(),
		ExtractTo:  extractPath,
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
	}, nil
}

// parseReference splits an OCI reference into registry, repository, and tag.
// e.g., "ghcr.io/org/skills/my-skill:1.0.0" -> ("ghcr.io", "org/skills/my-skill", "1.0.0")
// e.g., "localhost:5000/my-skill:1.0.0" -> ("localhost:5000", "my-skill", "1.0.0")
func parseReference(ref string) (registry, repository, tag string) {
	tag = "latest"

	// Handle digest references
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		tag = ref[idx+1:]
		ref = ref[:idx]
	} else if idx := strings.LastIndex(ref, ":"); idx > 0 {
		possibleTag := ref[idx+1:]
		// If it doesn't contain a slash, it's a tag not a port
		if !strings.Contains(possibleTag, "/") {
			tag = possibleTag
			ref = ref[:idx]
		}
	}

	// Split registry from repository
	// The registry is the first component if it contains a dot or colon, or is "localhost"
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 1 {
		// No slash, entire ref is the repository on docker.io
		registry = "docker.io"
		repository = parts[0]
	} else if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost" {
		registry = parts[0]
		repository = parts[1]
	} else {
		// No dot/colon in first part, treat as docker.io namespace
		registry = "docker.io"
		repository = ref
	}

	return registry, repository, tag
}

// copyDir recursively copies src to dst. Used on Windows where symlinks are
// not always available without elevated privileges.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}

// extractTarGz extracts a tar.gz stream to the given output directory.
func extractTarGz(r io.Reader, outputDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(outputDir, filepath.Clean(header.Name))
		if !strings.HasPrefix(target, filepath.Clean(outputDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar entry path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("creating file %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing file %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}
