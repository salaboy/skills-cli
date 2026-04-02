package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Archive creates a deterministic tar+gzip archive of the skill directory.
// The archive is rooted at <skill-name>/ and uses fixed timestamps, sorted
// entries, and zero uid/gid for reproducible digests.
func Archive(sd *SkillDirectory) ([]byte, error) {
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("creating gzip writer: %w", err)
	}
	tw := tar.NewWriter(gw)

	// Collect all files relative to the skill directory
	var entries []string
	err = filepath.Walk(sd.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sd.Path, path)
		if err != nil {
			return err
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking skill directory: %w", err)
	}

	// Sort for determinism
	sort.Strings(entries)

	epoch := time.Unix(0, 0)

	for _, rel := range entries {
		fullPath := filepath.Join(sd.Path, rel)
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", rel, err)
		}

		// Archive path: <skill-name>/<relative-path>
		archivePath := filepath.Join(sd.Config.Name, rel)
		if rel == "." {
			archivePath = sd.Config.Name + "/"
		}

		header := &tar.Header{
			Name:    filepath.ToSlash(archivePath),
			ModTime: epoch,
			Uid:     0,
			Gid:     0,
		}

		if info.IsDir() {
			header.Typeflag = tar.TypeDir
			header.Mode = 0755
			if !bytes.HasSuffix([]byte(header.Name), []byte("/")) {
				header.Name += "/"
			}
			if err := tw.WriteHeader(header); err != nil {
				return nil, fmt.Errorf("writing dir header %s: %w", rel, err)
			}
			continue
		}

		header.Typeflag = tar.TypeReg
		header.Mode = 0644
		header.Size = info.Size()

		// Preserve executable bit for scripts
		if info.Mode()&0111 != 0 {
			header.Mode = 0755
		}

		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("writing file header %s: %w", rel, err)
		}

		f, err := os.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", rel, err)
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("writing %s: %w", rel, err)
		}
		f.Close()
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}
