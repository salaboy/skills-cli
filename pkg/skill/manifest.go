package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ManifestFile = "skills.json"
	LockFile     = "skills.lock.json"
)

// LoadManifest reads skills.json from the given directory.
// Returns an empty manifest if the file doesn't exist.
func LoadManifest(dir string) (*SkillsManifest, error) {
	path := filepath.Join(dir, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsManifest{Skills: []SkillDependency{}}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", ManifestFile, err)
	}

	var m SkillsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ManifestFile, err)
	}
	return &m, nil
}

// SaveManifest writes skills.json to the given directory.
func SaveManifest(dir string, m *SkillsManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", ManifestFile, err)
	}
	data = append(data, '\n')

	path := filepath.Join(dir, ManifestFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", ManifestFile, err)
	}
	return nil
}

// AddToManifest adds or updates a skill entry in the manifest.
func AddToManifest(m *SkillsManifest, name, source, version string) {
	for i, s := range m.Skills {
		if s.Name == name {
			m.Skills[i].Source = source
			m.Skills[i].Version = version
			return
		}
	}
	m.Skills = append(m.Skills, SkillDependency{
		Name:    name,
		Source:   source,
		Version: version,
	})
}

// LoadLock reads skills.lock.json from the given directory.
// Returns an empty lock if the file doesn't exist.
func LoadLock(dir string) (*SkillsLock, error) {
	path := filepath.Join(dir, LockFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SkillsLock{
				LockfileVersion: 1,
				GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
				Skills:          []LockedSkill{},
			}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", LockFile, err)
	}

	var l SkillsLock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", LockFile, err)
	}
	return &l, nil
}

// SaveLock writes skills.lock.json to the given directory.
func SaveLock(dir string, l *SkillsLock) error {
	l.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", LockFile, err)
	}
	data = append(data, '\n')

	path := filepath.Join(dir, LockFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", LockFile, err)
	}
	return nil
}

// AddToLock adds or updates a skill entry in the lock file.
func AddToLock(l *SkillsLock, entry LockedSkill) {
	for i, s := range l.Skills {
		if s.Name == entry.Name {
			l.Skills[i] = entry
			return
		}
	}
	l.Skills = append(l.Skills, entry)
}

// RemoveFromManifest removes a skill entry by name.
func RemoveFromManifest(m *SkillsManifest, name string) bool {
	for i, s := range m.Skills {
		if s.Name == name {
			m.Skills = append(m.Skills[:i], m.Skills[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveFromLock removes a skill entry by name.
func RemoveFromLock(l *SkillsLock, name string) bool {
	for i, s := range l.Skills {
		if s.Name == name {
			l.Skills = append(l.Skills[:i], l.Skills[i+1:]...)
			return true
		}
	}
	return false
}
