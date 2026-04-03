package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/salaboy/skills-oci/pkg/skill"
	"oras.land/oras-go/v2/registry/remote"
)

// PushCollectionOptions configures a collection push operation.
type PushCollectionOptions struct {
	Reference string   // e.g., "ghcr.io/myorg/collections/dev-tools"
	Tag       string   // e.g., "v1.0.0"
	Name      string   // collection name (io.agentskills.collection.name)
	SkillRefs []string // OCI references of skills to include
	PlainHTTP bool

	OnStatus func(phase string)
}

// PushCollectionResult is returned after a successful collection push.
type PushCollectionResult struct {
	Digest    string
	Reference string
	Tag       string
	Size      int64
	Skills    int // number of skills in the collection
}

// FetchCollectionOptions configures a collection fetch operation.
type FetchCollectionOptions struct {
	Reference string
	PlainHTTP bool

	OnStatus func(phase string)
}

// PushCollection builds an OCI Image Index referencing existing skill artifacts
// and pushes it to a remote registry.
func PushCollection(ctx context.Context, opts PushCollectionOptions) (*PushCollectionResult, error) {
	if opts.OnStatus != nil {
		opts.OnStatus("Resolving skill references")
	}

	var manifests []ocispec.Descriptor
	for _, skillRef := range opts.SkillRefs {
		if opts.OnStatus != nil {
			opts.OnStatus(fmt.Sprintf("Resolving %s", skillRef))
		}
		desc, err := resolveSkillDescriptor(ctx, skillRef, opts.PlainHTTP)
		if err != nil {
			return nil, fmt.Errorf("resolving skill %s: %w", skillRef, err)
		}
		manifests = append(manifests, desc)
	}

	if opts.OnStatus != nil {
		opts.OnStatus("Building collection index")
	}

	tag := opts.Tag
	if tag == "" {
		tag = "latest"
	}

	collectionAnnotations := map[string]string{
		AnnotationCollectionName:  opts.Name,
		ocispec.AnnotationCreated: time.Now().UTC().Format(time.RFC3339),
		ocispec.AnnotationTitle:   opts.Name,
	}

	index := ocispec.Index{
		Versioned:    specs.Versioned{SchemaVersion: 2},
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: CollectionArtifactType,
		Manifests:    manifests,
		Annotations:  collectionAnnotations,
	}

	indexJSON, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("marshaling index: %w", err)
	}

	indexDesc := ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: CollectionArtifactType,
		Digest:       digest.FromBytes(indexJSON),
		Size:         int64(len(indexJSON)),
		Annotations:  collectionAnnotations,
	}

	if opts.OnStatus != nil {
		opts.OnStatus("Pushing collection to registry")
	}

	repo, err := remote.NewRepository(opts.Reference)
	if err != nil {
		return nil, fmt.Errorf("creating remote repository: %w", err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	repo.Client = NewAuthClient()

	if err := repo.Push(ctx, indexDesc, bytes.NewReader(indexJSON)); err != nil {
		return nil, fmt.Errorf("pushing collection index: %w", err)
	}

	if err := repo.Tag(ctx, indexDesc, tag); err != nil {
		return nil, fmt.Errorf("tagging collection: %w", err)
	}

	return &PushCollectionResult{
		Digest:    indexDesc.Digest.String(),
		Reference: opts.Reference,
		Tag:       tag,
		Size:      indexDesc.Size,
		Skills:    len(manifests),
	}, nil
}

// FetchCollection fetches an OCI Image Index from the registry and returns
// the collection metadata and the list of referenced skill descriptors.
func FetchCollection(ctx context.Context, opts FetchCollectionOptions) (*skill.FetchCollectionResult, error) {
	if opts.OnStatus != nil {
		opts.OnStatus("Resolving collection reference")
	}

	_, _, tag := parseReference(opts.Reference)

	repo, err := remote.NewRepository(opts.Reference)
	if err != nil {
		return nil, fmt.Errorf("creating remote repository: %w", err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	repo.Client = NewAuthClient()

	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("resolving collection: %w", err)
	}

	if opts.OnStatus != nil {
		opts.OnStatus("Fetching collection index")
	}

	reader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching collection index: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading collection index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing collection index: %w", err)
	}

	if index.ArtifactType != CollectionArtifactType {
		return nil, fmt.Errorf("artifact at %s is not a skills collection (artifactType: %q)", opts.Reference, index.ArtifactType)
	}

	result := &skill.FetchCollectionResult{
		Name:    index.Annotations[AnnotationCollectionName],
		Version: index.Annotations[ocispec.AnnotationVersion],
	}

	for _, m := range index.Manifests {
		entry := skill.CollectionSkillRef{
			Name:        m.Annotations[AnnotationSkillName],
			Ref:         m.Annotations[AnnotationSkillRef],
			Digest:      m.Digest.String(),
			Version:     m.Annotations[ocispec.AnnotationVersion],
			Description: m.Annotations[ocispec.AnnotationDescription],
		}
		result.Skills = append(result.Skills, entry)
	}

	return result, nil
}

// resolveSkillDescriptor connects to the registry, resolves the skill manifest,
// reads its annotations, and returns a descriptor suitable for embedding in a
// collection index.
func resolveSkillDescriptor(ctx context.Context, ref string, plainHTTP bool) (ocispec.Descriptor, error) {
	registry, repository, tag := parseReference(ref)

	repoRef := fmt.Sprintf("%s/%s", registry, repository)
	repo, err := remote.NewRepository(repoRef)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("creating repository: %w", err)
	}
	repo.PlainHTTP = plainHTTP
	repo.Client = NewAuthClient()

	// Resolve the tag to get digest and size.
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("resolving %s: %w", ref, err)
	}

	// Fetch the manifest to extract skill-level annotations.
	manifestReader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("fetching manifest for %s: %w", ref, err)
	}
	defer manifestReader.Close()

	manifestData, err := io.ReadAll(manifestReader)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("reading manifest for %s: %w", ref, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("parsing manifest for %s: %w", ref, err)
	}

	// Build descriptor-level annotations for the collection entry.
	annotations := map[string]string{
		AnnotationSkillRef: ref,
	}
	if name := manifest.Annotations[AnnotationSkillName]; name != "" {
		annotations[AnnotationSkillName] = name
	}
	if title := manifest.Annotations[ocispec.AnnotationTitle]; title != "" {
		annotations[ocispec.AnnotationTitle] = title
	}
	if version := manifest.Annotations[ocispec.AnnotationVersion]; version != "" {
		annotations[ocispec.AnnotationVersion] = version
	}
	if description := manifest.Annotations[ocispec.AnnotationDescription]; description != "" {
		annotations[ocispec.AnnotationDescription] = description
	}

	return ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: ArtifactType,
		Digest:       desc.Digest,
		Size:         desc.Size,
		Annotations:  annotations,
	}, nil
}
