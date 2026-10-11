package clean

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ollamaGC removes blobs no manifest references any more, the same prune
// Ollama runs itself. Needed when several tags sharing weights are removed
// together: each one alone saw its blobs as shared and left them in place.
func ollamaGC(manifestPath string) {
	i := strings.LastIndex(manifestPath, string(filepath.Separator)+"manifests"+string(filepath.Separator))
	if i < 0 {
		return
	}
	root := manifestPath[:i]
	// Drop the now-empty model directory, if any.
	os.Remove(filepath.Dir(manifestPath))

	used := map[string]bool{}
	ok := true
	filepath.WalkDir(filepath.Join(root, "manifests"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			ok = false
			return nil
		}
		if d.IsDir() {
			return nil
		}
		var m struct {
			Config struct{ Digest string }   `json:"config"`
			Layers []struct{ Digest string } `json:"layers"`
		}
		data, err := os.ReadFile(p)
		if err != nil || json.Unmarshal(data, &m) != nil {
			ok = false // an unreadable manifest might reference anything; don't GC
			return nil
		}
		used[strings.Replace(m.Config.Digest, ":", "-", 1)] = true
		for _, l := range m.Layers {
			used[strings.Replace(l.Digest, ":", "-", 1)] = true
		}
		return nil
	})
	if !ok {
		return
	}
	entries, err := os.ReadDir(filepath.Join(root, "blobs"))
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		// Leave in-progress downloads (sha256-…-partial*) alone.
		if !e.Type().IsRegular() || !strings.HasPrefix(n, "sha256-") || strings.Contains(n, "partial") || used[n] {
			continue
		}
		os.Remove(filepath.Join(root, "blobs", n))
	}
}
