package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/salaboy/skills-cli/pkg/skill"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

// PullOptions configures a pull operation.
type PullOptions struct {
	Reference string // Full OCI reference, e.g., "ghcr.io/org/skills/my-skill:1.0.0"
	OutputDir string // Directory to extract skill into (default: .agents/skills)
	PlainHTTP bool

	OnStatus func(phase string)
}

// PullResult is returned after a successful pull.
type PullResult struct {
	Name      string
	Version   string
	Digest    string
	ExtractTo string
}

// Pull fetches a skill artifact from a remote registry and extracts it.
func Pull(ctx context.Context, opts PullOptions) (*PullResult, error) {
	if opts.OnStatus != nil {
		opts.OnStatus("Resolving reference")
	}

	// Parse the reference to separate repo from tag/digest
	ref := opts.Reference
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("creating remote repository: %w", err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	repo.Client = NewAuthClient()

	// Extract tag from reference
	tag := "latest"
	if idx := strings.LastIndex(ref, ":"); idx > 0 {
		possibleTag := ref[idx+1:]
		// Check it's not a port number (contains /)
		if !strings.Contains(possibleTag, "/") {
			tag = possibleTag
		}
	}
	// Handle digest references
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		tag = ref[idx+1:]
	}

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

	return &PullResult{
		Name:      skillConfig.Name,
		Version:   skillConfig.Version,
		Digest:    desc.Digest.String(),
		ExtractTo: extractPath,
	}, nil
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
