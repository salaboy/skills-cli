package skill

// SkillConfig is the OCI config JSON for a skill artifact.
// Fields map to the Agent Skills OCI Artifacts Spec v0.1.0 config schema.
type SkillConfig struct {
	SchemaVersion string            `json:"schemaVersion" yaml:"-"`
	Name          string            `json:"name" yaml:"name"`
	Version       string            `json:"version,omitempty" yaml:"version"`
	Description   string            `json:"description,omitempty" yaml:"description"`
	License       string            `json:"license,omitempty" yaml:"license"`
	Compatibility string            `json:"compatibility,omitempty" yaml:"compatibility"`
	AllowedTools  []string          `json:"allowedTools,omitempty" yaml:"allowedTools"`
	Metadata      map[string]any    `json:"metadata,omitempty" yaml:"metadata"`
}

// SkillDirectory represents a parsed skill on disk.
type SkillDirectory struct {
	Path     string      // Absolute path to skill directory
	Config   SkillConfig // Parsed from SKILL.md frontmatter
	Markdown string      // Body of SKILL.md (after frontmatter)
}

// SkillsManifest represents skills.json — the declarative manifest.
type SkillsManifest struct {
	Skills []SkillDependency `json:"skills"`
}

// SkillDependency is a single entry in skills.json.
type SkillDependency struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
}

// SkillsLock represents skills.lock.json — the lock file for reproducibility.
type SkillsLock struct {
	LockfileVersion int           `json:"lockfileVersion"`
	GeneratedAt     string        `json:"generatedAt"`
	Skills          []LockedSkill `json:"skills"`
}

// LockedSkill is a single entry in skills.lock.json.
type LockedSkill struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Source      LockSource  `json:"source"`
	InstalledAt string      `json:"installedAt"`
}

// LockSource holds the resolved OCI reference for a locked skill.
type LockSource struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
	Ref        string `json:"ref"`
}
