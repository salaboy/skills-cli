package oci

const (
	// ArtifactType is the OCI artifact type for a skill.
	ArtifactType = "application/vnd.agentskills.skill.v1"

	// ConfigMediaType is the media type for the skill config JSON blob.
	ConfigMediaType = "application/vnd.agentskills.skill.config.v1+json"

	// ContentMediaType is the media type for the skill content layer (tar+gzip).
	ContentMediaType = "application/vnd.agentskills.skill.content.v1.tar+gzip"

	// CatalogType is the OCI artifact type for a skills catalog.
	CatalogType = "application/vnd.agentskills.catalog.v1"

	// AnnotationSkillName is the annotation key for the skill name.
	AnnotationSkillName = "io.agentskills.skill.name"

	// AnnotationSkillCompatibility is the annotation key for compatibility notes.
	AnnotationSkillCompatibility = "io.agentskills.skill.compatibility"
)
