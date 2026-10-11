package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

type ollamaManifest struct {
	Config ollamaLayer   `json:"config"`
	Layers []ollamaLayer `json:"layers"`
}

type ollamaLayer struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// ollamaItems reads Ollama's manifests directly. Blobs are content-addressed
// and shared between tags (llama3:latest and llama3:8b are the same weights),
// so each model's freeable size counts only blobs no other manifest uses.
func ollamaItems(env *rules.Env) []*core.Item {
	r := rules.ByID("ai.ollama")
	var out []*core.Item
	for _, root := range rules.OllamaDirs(env) {
		out = append(out, ollamaStore(env, r, root)...)
	}
	return out
}

func ollamaStore(env *rules.Env, r *rules.Rule, root string) []*core.Item {
	manifestRoot := filepath.Join(root, "manifests")
	type model struct {
		name, path string
		digests    map[string]int64
	}
	var models []*model
	refs := map[string]int{}

	filepath.WalkDir(manifestRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var m ollamaManifest
		if json.Unmarshal(data, &m) != nil {
			return nil
		}
		rel, _ := filepath.Rel(manifestRoot, p)
		mod := &model{name: ollamaName(filepath.ToSlash(rel)), path: p, digests: map[string]int64{}}
		for _, l := range append(m.Layers, m.Config) {
			if l.Digest != "" {
				mod.digests[l.Digest] = l.Size
			}
		}
		for dg := range mod.digests {
			refs[dg]++
		}
		models = append(models, mod)
		return nil
	})

	hasCLI := env.LookPath != nil && func() bool { _, err := env.LookPath("ollama"); return err == nil }()
	var out []*core.Item
	for _, m := range models {
		it := &core.Item{
			ID: "ollama:" + m.path, RuleID: r.ID, Ecosystem: r.Ecosystem, Source: r.Source,
			Name: m.name, Safety: r.Safety, Why: r.Why, Restore: "ollama pull " + m.name,
			Paths: []string{m.path}, SizeKnown: true,
		}
		if fi, err := os.Stat(m.path); err == nil {
			it.ModTime = fi.ModTime()
		}
		for dg, size := range m.digests {
			blob := filepath.Join(root, "blobs", strings.Replace(dg, ":", "-", 1))
			if _, err := os.Stat(blob); err != nil {
				continue
			}
			it.Size += size
			if refs[dg] > 1 {
				it.Shared += size
			} else {
				it.Paths = append(it.Paths, blob)
			}
		}
		if hasCLI {
			it.Command = []string{"ollama", "rm", m.name}
		}
		if it.Shared > 0 {
			it.Note = "shares " + core.HumanBytes(it.Shared) + " of blobs with other local models"
		}
		out = append(out, it)
	}
	return out
}

// ollamaName turns a manifest path into the name `ollama list` shows.
func ollamaName(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) < 4 {
		return rel
	}
	host, ns, model, tag := parts[0], parts[1], parts[2], parts[3]
	switch {
	case host == "registry.ollama.ai" && ns == "library":
		return model + ":" + tag
	case host == "registry.ollama.ai":
		return ns + "/" + model + ":" + tag
	}
	return host + "/" + ns + "/" + model + ":" + tag
}
