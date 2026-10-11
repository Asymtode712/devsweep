package clean

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

func env(home string) *rules.Env {
	return &rules.Env{GOOS: "linux", Home: home, Getenv: func(k string) string {
		if k == "XDG_STATE_HOME" {
			return filepath.Join(home, "state")
		}
		return ""
	}}
}

func TestRefusesProtectedPaths(t *testing.T) {
	home := t.TempDir()
	e := env(home)
	for _, p := range []string{home, "/", "/usr", filepath.Join(home, ".local"), "relative/path", home + "/a/../b"} {
		it := &core.Item{RuleID: "node.npm-cache", Paths: []string{p}}
		if err := Validate(e, it); err == nil {
			t.Errorf("Validate allowed %q", p)
		}
	}
}

func TestRefusesSymlinkSwap(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "precious")
	os.MkdirAll(target, 0o755)
	cache := filepath.Join(home, "cache", "pip")
	os.MkdirAll(filepath.Dir(cache), 0o755)
	if err := os.Symlink(target, cache); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	it := &core.Item{RuleID: "python.pip-cache", Paths: []string{cache}}
	if err := Validate(env(home), it); err == nil {
		t.Fatal("Validate allowed a path that became a symlink")
	}
}

func TestRevalidatesProjectManifest(t *testing.T) {
	home := t.TempDir()
	nm := filepath.Join(home, "app", "node_modules")
	os.MkdirAll(nm, 0o755)
	it := &core.Item{RuleID: "node.modules", Paths: []string{nm}}
	if err := Validate(env(home), it); err == nil {
		t.Fatal("node_modules without package.json should be refused")
	}
	os.WriteFile(filepath.Join(home, "app", "package.json"), []byte("{}"), 0o644)
	if err := Validate(env(home), it); err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}
}

func TestRefusesGitTracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home := t.TempDir()
	app := filepath.Join(home, "app")
	os.MkdirAll(filepath.Join(app, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(app, "package.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(app, "node_modules", "vendored.js"), []byte("x"), 0o644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = app
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	it := &core.Item{RuleID: "node.modules", Paths: []string{filepath.Join(app, "node_modules")}}
	res := Clean(env(home), it, Options{})
	if res.Err == nil {
		t.Fatal("deleted a git-tracked node_modules")
	}
	if _, err := os.Stat(filepath.Join(app, "node_modules", "vendored.js")); err != nil {
		t.Fatal("tracked file is gone")
	}
}

func TestRemovesReadOnlyTrees(t *testing.T) {
	home := t.TempDir()
	mod := filepath.Join(home, "go", "pkg", "mod", "example.com", "x@v1")
	os.MkdirAll(mod, 0o755)
	os.WriteFile(filepath.Join(mod, "x.go"), []byte("package x"), 0o444)
	os.Chmod(mod, 0o555) // Go's module cache is read-only like this
	target := filepath.Join(home, "go", "pkg", "mod")
	it := &core.Item{RuleID: "go.mod-cache", Paths: []string{target}}
	if res := Clean(env(home), it, Options{}); res.Err != nil {
		t.Fatal(res.Err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("module cache still exists")
	}
}

func TestOllamaGC(t *testing.T) {
	root := t.TempDir()
	m := filepath.Join(root, "manifests", "registry.ollama.ai", "library")
	os.MkdirAll(filepath.Join(m, "keep"), 0o755)
	os.MkdirAll(filepath.Join(root, "blobs"), 0o755)
	os.WriteFile(filepath.Join(m, "keep", "latest"), []byte(`{"config":{"digest":"sha256:c"},"layers":[{"digest":"sha256:k"}]}`), 0o644)
	for _, b := range []string{"sha256-c", "sha256-k", "sha256-orphan", "sha256-dl-partial"} {
		os.WriteFile(filepath.Join(root, "blobs", b), []byte("x"), 0o644)
	}
	ollamaGC(filepath.Join(m, "gone", "latest"))
	for b, want := range map[string]bool{"sha256-c": true, "sha256-k": true, "sha256-orphan": false, "sha256-dl-partial": true} {
		_, err := os.Stat(filepath.Join(root, "blobs", b))
		if (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", b, err == nil, want)
		}
	}
}
