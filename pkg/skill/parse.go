package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads a skill directory and returns a SkillDirectory with the parsed
// SKILL.md frontmatter and markdown body.
func Parse(skillPath string) (*SkillDirectory, error) {
	absPath, err := filepath.Abs(skillPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	skillMDPath := filepath.Join(absPath, "SKILL.md")
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}

	config, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing SKILL.md frontmatter: %w", err)
	}

	// Always set schema version per spec
	config.SchemaVersion = "1"

	// If name is not set in frontmatter, derive from directory name
	if config.Name == "" {
		config.Name = filepath.Base(absPath)
	}

	return &SkillDirectory{
		Path:     absPath,
		Config:   *config,
		Markdown: body,
	}, nil
}

// parseFrontmatter splits a SKILL.md file into YAML frontmatter and markdown body.
// Frontmatter is delimited by --- on its own line at the start of the file.
func parseFrontmatter(content string) (*SkillConfig, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, "", fmt.Errorf("SKILL.md must start with --- frontmatter delimiter")
	}

	// Find the closing ---
	rest := content[3:]
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, "", fmt.Errorf("SKILL.md missing closing --- frontmatter delimiter")
	}

	yamlContent := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:])

	var config SkillConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		return nil, "", fmt.Errorf("unmarshaling frontmatter YAML: %w", err)
	}

	return &config, body, nil
}
