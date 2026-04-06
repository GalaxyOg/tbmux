package discovery

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tbmux/internal/config"
	"tbmux/internal/model"
)

type Scanner struct {
	ExcludePatterns []string
	RunningWindow   time.Duration
	Now             func() time.Time
}

type candidate struct {
	sourcePath string
	watchRoot  string
	alias      string
	lastMod    time.Time
}

func NewScanner(exclude []string, runningWindow time.Duration) Scanner {
	return Scanner{
		ExcludePatterns: exclude,
		RunningWindow:   runningWindow,
		Now:             time.Now,
	}
}

func (s Scanner) Discover(roots []config.WatchedRoot) ([]model.RunRecord, error) {
	now := s.Now()
	candByDir := map[string]candidate{}
	for _, root := range roots {
		rootPath := filepath.Clean(root.Path)
		st, err := os.Stat(rootPath)
		if err != nil || !st.IsDir() {
			continue
		}
		alias := strings.TrimSpace(root.Alias)
		if alias == "" {
			alias = filepath.Base(rootPath)
		}
		walkErr := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if s.shouldExclude(path) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !isEventFilename(d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			dir := filepath.Dir(path)
			old, ok := candByDir[dir]
			if !ok || info.ModTime().After(old.lastMod) {
				candByDir[dir] = candidate{
					sourcePath: dir,
					watchRoot:  rootPath,
					alias:      alias,
					lastMod:    info.ModTime(),
				}
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	out := make([]model.RunRecord, 0, len(candByDir))
	for _, cand := range candByDir {
		rel, err := filepath.Rel(cand.watchRoot, cand.sourcePath)
		if err != nil {
			rel = filepath.Base(cand.sourcePath)
		}
		name := buildReadableName(cand.alias, rel)
		id := stableID(cand.sourcePath)
		out = append(out, model.RunRecord{
			ID:            id,
			Name:          name,
			SourcePath:    cand.sourcePath,
			WatchRoot:     cand.watchRoot,
			LastUpdatedAt: cand.lastMod.UTC(),
			IsRunning:     now.Sub(cand.lastMod) <= s.RunningWindow,
		})
	}
	applyNameCollisionSuffix(out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastUpdatedAt.Equal(out[j].LastUpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].LastUpdatedAt.After(out[j].LastUpdatedAt)
	})
	return out, nil
}

func (s Scanner) shouldExclude(path string) bool {
	if len(s.ExcludePatterns) == 0 {
		return false
	}
	norm := filepath.ToSlash(path)
	for _, p := range s.ExcludePatterns {
		p = strings.TrimSpace(filepath.ToSlash(p))
		if p == "" {
			continue
		}
		if ok, _ := filepath.Match(p, norm); ok {
			return true
		}
		if ok, _ := filepath.Match(p, filepath.Base(norm)); ok {
			return true
		}
		t := strings.Trim(p, "*")
		if t != "" && strings.Contains(norm, t) {
			return true
		}
	}
	return false
}

func isEventFilename(name string) bool {
	if strings.HasPrefix(name, "events.out.tfevents.") {
		return true
	}
	if strings.Contains(name, ".tfevents.") {
		return true
	}
	return false
}

func stableID(path string) string {
	sum := sha1.Sum([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:])[:12]
}

func buildReadableName(alias, rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	if rel == "." || rel == "" {
		rel = "root"
	}
	joined := alias + "__" + rel
	var b strings.Builder
	for _, r := range joined {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		case r == '/':
			b.WriteString("__")
		default:
			b.WriteRune('_')
		}
	}
	name := b.String()
	name = strings.Trim(name, "_")
	if name == "" {
		return "run"
	}
	return name
}

func applyNameCollisionSuffix(runs []model.RunRecord) {
	nameCount := map[string]int{}
	for _, run := range runs {
		nameCount[run.Name]++
	}
	for i := range runs {
		if nameCount[runs[i].Name] > 1 {
			runs[i].Name = fmt.Sprintf("%s__%s", runs[i].Name, runs[i].ID[:6])
		}
	}
}
