package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/salaboy/skills-oci/pkg/skill"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

// PushOptions configures a push operation.
type PushOptions struct {
	Reference string // e.g., "ghcr.io/org/skills/my-skill"
	Tag       string // e.g., "1.0.0"
	SkillDir  string // path to skill directory
	PlainHTTP bool   // use HTTP instead of HTTPS (for local registries)

	// OnStatus is called with status updates during the push workflow.
	OnStatus func(phase string)
}

// PushResult is returned after a successful push.
type PushResult struct {
	Digest    string
	Reference string
	Tag       string
	Size      int64
}

// Push packages a skill directory and pushes it as an OCI artifact to a remote registry.
func Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	if opts.OnStatus != nil {
		opts.OnStatus("Validating skill directory")
	}

	// 1. Validate
	if err := skill.Validate(opts.SkillDir); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 2. Parse SKILL.md
	if opts.OnStatus != nil {
		opts.OnStatus("Parsing SKILL.md")
	}
	sd, err := skill.Parse(opts.SkillDir)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	if err := skill.ValidateName(sd.Config.Name); err != nil {
		return nil, err
	}

	// Override version from tag if provided
	if opts.Tag != "" {
		sd.Config.Version = opts.Tag
	}

	// 3. Create deterministic archive
	if opts.OnStatus != nil {
		opts.OnStatus("Creating archive")
	}
	archiveBytes, err := skill.Archive(sd)
	if err != nil {
		return nil, fmt.Errorf("archiving failed: %w", err)
	}

	// 4. Marshal config JSON
	configJSON, err := json.Marshal(sd.Config)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	// 5. Build OCI artifact in memory store
	if opts.OnStatus != nil {
		opts.OnStatus("Pushing to registry")
	}
	store := memory.New()

	// Push config blob
	configDesc := ocispec.Descriptor{
		MediaType: ConfigMediaType,
		Digest:    digest.FromBytes(configJSON),
		Size:      int64(len(configJSON)),
	}
	if err := store.Push(ctx, configDesc, bytes.NewReader(configJSON)); err != nil {
		return nil, fmt.Errorf("storing config: %w", err)
	}

	// Push content layer
	layerDesc := ocispec.Descriptor{
		MediaType: ContentMediaType,
		Digest:    digest.FromBytes(archiveBytes),
		Size:      int64(len(archiveBytes)),
	}
	if err := store.Push(ctx, layerDesc, bytes.NewReader(archiveBytes)); err != nil {
		return nil, fmt.Errorf("storing layer: %w", err)
	}

	// Build annotations
	annotations := map[string]string{
		ocispec.AnnotationCreated: time.Now().UTC().Format(time.RFC3339),
		ocispec.AnnotationTitle:   sd.Config.Name,
		AnnotationSkillName:       sd.Config.Name,
	}
	if sd.Config.Version != "" {
		annotations[ocispec.AnnotationVersion] = sd.Config.Version
	}
	if sd.Config.Description != "" {
		annotations[ocispec.AnnotationDescription] = sd.Config.Description
	}
	if sd.Config.Compatibility != "" {
		annotations[AnnotationSkillCompatibility] = sd.Config.Compatibility
	}
	if sd.Config.License != "" {
		annotations["org.opencontainers.image.licenses"] = sd.Config.License
	}

	// Pack manifest
	packOpts := oras.PackManifestOptions{
		ConfigDescriptor:    &configDesc,
		Layers:              []ocispec.Descriptor{layerDesc},
		ManifestAnnotations: annotations,
	}

	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, ArtifactType, packOpts)
	if err != nil {
		return nil, fmt.Errorf("packing manifest: %w", err)
	}

	// Resolve tag: prefer explicit opts.Tag, then fall back to any tag
	// embedded in opts.Reference (e.g. "docker.io/org/skill:1.0.0"), then
	// default to "latest".
	tag := opts.Tag
	repoRef := opts.Reference
	if tag == "" {
		reg, rep, parsedTag := parseReference(opts.Reference)
		tag = parsedTag
		repoRef = reg + "/" + rep
	}
	if tag == "" {
		tag = "latest"
	}

	// Tag in local store
	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return nil, fmt.Errorf("tagging: %w", err)
	}

	// 6. Set up remote repository — use the tag-free reference so that the
	// tag passed to oras.Copy is the sole source of truth.
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		return nil, fmt.Errorf("creating remote repository: %w", err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	repo.Client = NewAuthClient()

	// 7. Copy from memory to remote
	desc, err := oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pushing to registry: %w", err)
	}

	return &PushResult{
		Digest:    desc.Digest.String(),
		Reference: opts.Reference,
		Tag:       tag,
		Size:      desc.Size,
	}, nil
}
