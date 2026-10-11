package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testEnv(home string) *rules.Env {
	return &rules.Env{GOOS: "linux", Home: home, Getenv: func(string) string { return "" },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
}

func byPath(items []*core.Item) map[string]*core.Item {
	m := map[string]*core.Item{}
	for _, it := range items {
		m[it.Paths[0]] = it
	}
	return m
}

func TestProjectRules(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "web/package.json"), 2)
	write(t, filepath.Join(root, "web/package-lock.json"), 2)
	write(t, filepath.Join(root, "web/node_modules/x/index.js"), 8192)
	write(t, filepath.Join(root, "nolock/package.json"), 2)
	write(t, filepath.Join(root, "nolock/node_modules/x/a"), 8192)
	write(t, filepath.Join(root, "rs/Cargo.toml"), 1)
	write(t, filepath.Join(root, "rs/target/debug/bin"), 8192)
	write(t, filepath.Join(root, "notrust/target/keep.txt"), 8192)
	write(t, filepath.Join(root, "py/requirements.txt"), 1)
	write(t, filepath.Join(root, "py/.venv/pyvenv.cfg"), 1)
	write(t, filepath.Join(root, "py/.venv/lib/x"), 8192)
	write(t, filepath.Join(root, "loose/env/pyvenv.cfg"), 1)
	write(t, filepath.Join(root, "cpp/build/CMakeCache.txt"), 1)
	write(t, filepath.Join(root, "plainbuild/build/out.bin"), 8192)
	// node_modules nested inside node_modules must not be descended into
	write(t, filepath.Join(root, "web/node_modules/dep/package.json"), 2)
	write(t, filepath.Join(root, "web/node_modules/dep/node_modules/y"), 8192)

	res := Run(testEnv(root), Options{Roots: []string{root}, NoGlobal: true})
	got := byPath(res.Items)

	expect := map[string]core.Safety{
		"web/node_modules":    core.Safe,
		"nolock/node_modules": core.Caution,
		"rs/target":           core.Safe,
		"py/.venv":            core.Caution,
		"loose/env":           core.Guarded,
		"cpp/build":           core.Safe,
	}
	for rel, safety := range expect {
		it := got[filepath.Join(root, rel)]
		if it == nil {
			t.Errorf("%s: not found", rel)
			continue
		}
		if it.Safety != safety {
			t.Errorf("%s: safety %v, want %v", rel, it.Safety, safety)
		}
	}
	for _, rel := range []string{"notrust/target", "plainbuild/build", "web/node_modules/dep/node_modules"} {
		if got[filepath.Join(root, rel)] != nil {
			t.Errorf("%s matched but must not", rel)
		}
	}
	if len(res.Items) != len(expect) {
		t.Errorf("got %d items, want %d", len(res.Items), len(expect))
	}
}

func TestHardlinksCountAsShared(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hardlink accounting is Unix-only")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "store/blob"), 64*1024)
	write(t, filepath.Join(root, "proj/node_modules/own"), 16*1024)
	if err := os.Link(filepath.Join(root, "store/blob"), filepath.Join(root, "proj/node_modules/linked")); err != nil {
		t.Skip("hardlinks unsupported:", err)
	}
	s := NewSizer(4)
	total, shared := s.Measure([]string{filepath.Join(root, "proj/node_modules")})
	if shared == 0 || shared >= total {
		t.Fatalf("total=%d shared=%d; expected the linked blob to be shared", total, shared)
	}
	// Measuring both sides together: every link is inside the set, nothing is shared.
	_, shared = s.Measure([]string{filepath.Join(root, "proj"), filepath.Join(root, "store")})
	if shared != 0 {
		t.Fatalf("shared=%d; want 0 when all links are inside", shared)
	}
}

func TestOllamaSharedBlobs(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(home, ".ollama/models")
	m := filepath.Join(store, "manifests/registry.ollama.ai/library")
	write(t, filepath.Join(store, "blobs/sha256-w"), 10)
	write(t, filepath.Join(store, "blobs/sha256-c"), 1)
	write(t, filepath.Join(store, "blobs/sha256-q"), 10)
	shared := `{"config":{"digest":"sha256:c","size":1},"layers":[{"digest":"sha256:w","size":1000}]}`
	write(t, filepath.Join(m, "llama3/latest"), 0)
	os.WriteFile(filepath.Join(m, "llama3/latest"), []byte(shared), 0o644)
	write(t, filepath.Join(m, "llama3/8b"), 0)
	os.WriteFile(filepath.Join(m, "llama3/8b"), []byte(shared), 0o644)
	write(t, filepath.Join(m, "qwen/7b"), 0)
	os.WriteFile(filepath.Join(m, "qwen/7b"), []byte(`{"config":{"digest":"sha256:c","size":1},"layers":[{"digest":"sha256:q","size":500}]}`), 0o644)

	items := ollamaItems(testEnv(home))
	byName := map[string]*core.Item{}
	for _, it := range items {
		byName[it.Name] = it
	}
	if it := byName["llama3:latest"]; it == nil || it.Frees() != 0 || it.Shared != 1001 {
		t.Errorf("llama3:latest = %+v; want everything shared", it)
	}
	// qwen shares only the config blob with llama3
	if it := byName["qwen:7b"]; it == nil || it.Frees() != 500 {
		t.Errorf("qwen:7b = %+v; want 500 freeable", it)
	}
}

func TestOllamaName(t *testing.T) {
	for in, want := range map[string]string{
		"registry.ollama.ai/library/llama3.1/70b": "llama3.1:70b",
		"registry.ollama.ai/someone/model/latest": "someone/model:latest",
		"hf.co/bartowski/Qwen-GGUF/Q4_K_M":        "hf.co/bartowski/Qwen-GGUF:Q4_K_M",
	} {
		if got := ollamaName(in); got != want {
			t.Errorf("ollamaName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestParseDockerSize(t *testing.T) {
	for in, want := range map[string]int64{"1.234GB (35%)": 1_234_000_000, "512MB": 512_000_000, "0B": 0, "12.5kB (100%)": 12_500} {
		if got := parseDockerSize(in); got != want {
			t.Errorf("parseDockerSize(%q) = %d; want %d", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{"~/code/app": "~/code/app", "~/Open Source/x": "~/'Open Source/x'", "/a b/c's": `'/a b/c'\''s'`} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q; want %q", in, got, want)
		}
	}
}
