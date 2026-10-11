// Package rules is the catalog of what devsweep knows how to clean, and the
// conditions under which each thing is considered safe to delete.
package rules

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Asymtode712/devsweep/internal/core"
)

type Kind int

const (
	// Project artifacts live inside a project and are only matched when the
	// project's manifest sits next to them (node_modules beside package.json).
	Project Kind = iota
	// Global caches live at well-known per-user locations.
	Global
	// Provider items come from a tool's own metadata (Docker, Ollama, Hugging Face).
	Provider
)

type Rule struct {
	ID        string
	Ecosystem string
	Source    string
	Name      string
	Kind      Kind
	Safety    core.Safety
	Why       string
	Restore   string

	// Project rules.
	Dirs        []string          // directory names to match
	Markers     []string          // the parent must contain one of these (globs allowed); empty = no requirement
	InnerMarker string            // the matched dir must contain this entry
	LockFiles   []string          // if none exist next to the dir, safety drops a level
	NoLockNote  string            // shown when LockFiles are missing
	LockRestore map[string]string // lock file → restore command

	// Global rules.
	Paths      func(e *Env) []string
	PerChild   bool // every child directory becomes its own item
	ChildDepth int  // with PerChild: items sit this many levels down (default 1)
	ChildName  func(name string) string
	Command    func(e *Env) []string // a native cleanup command preferred over deleting

	OS []string // limit to these GOOS values; empty = everywhere
}

func (r *Rule) Supported(goos string) bool {
	if len(r.OS) == 0 {
		return true
	}
	for _, o := range r.OS {
		if o == goos {
			return true
		}
	}
	return false
}

// MatchMarker reports whether names (a directory listing) satisfies the rule's markers.
func (r *Rule) MatchMarker(names map[string]bool) bool {
	if len(r.Markers) == 0 {
		return true
	}
	for _, m := range r.Markers {
		if hasEntry(names, m) {
			return true
		}
	}
	return false
}

// HasLock reports the first lock file present, if the rule tracks lock files.
func (r *Rule) HasLock(names map[string]bool) (string, bool) {
	if len(r.LockFiles) == 0 {
		return "", true
	}
	for _, l := range r.LockFiles {
		if names[l] {
			return l, true
		}
	}
	return "", false
}

func hasEntry(names map[string]bool, pattern string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return names[pattern]
	}
	for n := range names {
		if ok, _ := filepath.Match(pattern, n); ok {
			return true
		}
	}
	return false
}

// Env resolves per-OS locations. Tests can construct one with a fake home.
type Env struct {
	GOOS         string
	Home         string
	UserCache    string // ~/Library/Caches on macOS, XDG cache on Linux, %LocalAppData% on Windows
	XDGCache     string // ~/.cache (or $XDG_CACHE_HOME); many cross-platform tools use it on macOS too
	LocalAppData string
	Getenv       func(string) string
	LookPath     func(string) (string, error)
}

func NewEnv() *Env {
	home, _ := os.UserHomeDir()
	e := &Env{GOOS: runtime.GOOS, Home: home, Getenv: os.Getenv, LookPath: exec.LookPath}
	e.UserCache, _ = os.UserCacheDir()
	e.XDGCache = filepath.Join(home, ".cache")
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" && runtime.GOOS != "windows" {
		e.XDGCache = v
	}
	e.LocalAppData = os.Getenv("LOCALAPPDATA")
	return e
}

func (e *Env) H(p ...string) string { return filepath.Join(append([]string{e.Home}, p...)...) }

// pick chooses a path by OS: mac, linux, windows. Empty strings are dropped.
func (e *Env) pick(mac, linux, windows string) []string {
	var p string
	switch e.GOOS {
	case "darwin":
		p = mac
	case "windows":
		p = windows
	default:
		p = linux
	}
	if p == "" {
		return nil
	}
	return []string{p}
}

func (e *Env) envOr(key string, def ...string) []string {
	if v := e.Getenv(key); v != "" {
		return filepath.SplitList(v)[:1]
	}
	return def
}

func (e *Env) has(bin string) bool {
	_, err := e.LookPath(bin)
	return err == nil
}

func (e *Env) lad(p ...string) string {
	if e.LocalAppData == "" {
		return ""
	}
	return filepath.Join(append([]string{e.LocalAppData}, p...)...)
}

var nodeLocks = map[string]string{
	"package-lock.json":   "npm ci",
	"npm-shrinkwrap.json": "npm ci",
	"yarn.lock":           "yarn install --frozen-lockfile",
	"pnpm-lock.yaml":      "pnpm install --frozen-lockfile",
	"bun.lockb":           "bun install --frozen-lockfile",
	"bun.lock":            "bun install --frozen-lockfile",
}

var pyManifests = map[string]string{
	"uv.lock":          "uv sync",
	"poetry.lock":      "poetry install",
	"Pipfile.lock":     "pipenv install",
	"requirements.txt": "python -m venv .venv && pip install -r requirements.txt",
	"pyproject.toml":   "python -m venv .venv && pip install -e .",
	"environment.yml":  "conda env create -f environment.yml",
	"setup.py":         "python -m venv .venv && pip install -e .",
}

// All returns the full catalog, in display order.
func All() []*Rule {
	return catalog
}

// ByID finds a rule.
func ByID(id string) *Rule {
	for _, r := range catalog {
		if r.ID == id {
			return r
		}
	}
	return nil
}

var catalog = []*Rule{
	// ---------------------------------------------------------------- AI models
	{ID: "ai.ollama", Ecosystem: "AI models", Source: "ollama", Name: "Ollama models", Kind: Provider, Safety: core.Caution,
		Why:     "Weights re-download from the registry, but large models are long pulls. Blobs are shared between tags, so removal goes through `ollama rm` (or devsweep's own blob refcount when Ollama isn't installed).",
		Restore: "ollama pull <model>"},
	{ID: "ai.huggingface", Ecosystem: "AI models", Source: "hf", Name: "Hugging Face hub cache", Kind: Provider, Safety: core.Caution,
		Why:     "Each models--org--name folder is a self-contained snapshot of a repo; it re-downloads on next use. Gated models need your token again.",
		Restore: "huggingface-cli download <repo>"},
	{ID: "ai.hf-datasets", Ecosystem: "AI models", Source: "hf", Name: "Hugging Face datasets cache", Kind: Global, Safety: core.Caution,
		Why:   "Arrow files written by the datasets library; regenerated from the hub on next load_dataset().",
		Paths: func(e *Env) []string { return e.envOr("HF_DATASETS_CACHE", filepath.Join(hfHome(e), "datasets")) }},
	{ID: "ai.hf-xet", Ecosystem: "AI models", Source: "hf", Name: "Hugging Face Xet chunk cache", Kind: Global, Safety: core.Safe,
		Why: "Deduplication chunk cache used during downloads; rebuilt on demand.",
		Paths: func(e *Env) []string {
			return []string{filepath.Join(hfHome(e), "xet")}
		}},
	{ID: "ai.lmstudio", Ecosystem: "AI models", Source: "lmstudio", Name: "LM Studio models", Kind: Global, Safety: core.Caution, PerChild: true, ChildDepth: 2,
		Why:     "Downloaded GGUF/MLX weights. Re-download from LM Studio's model browser.",
		Restore: "lms get <model>",
		Paths: func(e *Env) []string {
			return []string{e.H(".lmstudio", "models"), e.H(".cache", "lm-studio", "models")}
		}},
	{ID: "ai.torch", Ecosystem: "AI models", Source: "torch", Name: "PyTorch hub cache", Kind: Global, Safety: core.Safe,
		Why: "torch.hub checkpoints download automatically on next load.",
		Paths: func(e *Env) []string {
			return e.envOr("TORCH_HOME", filepath.Join(e.XDGCache, "torch"))
		}},
	{ID: "ai.whisper", Ecosystem: "AI models", Source: "whisper", Name: "Whisper model cache", Kind: Global, Safety: core.Safe,
		Why:   "openai-whisper re-downloads models on first use.",
		Paths: func(e *Env) []string { return []string{filepath.Join(e.XDGCache, "whisper")} }},
	{ID: "ai.llamacpp", Ecosystem: "AI models", Source: "llama.cpp", Name: "llama.cpp download cache", Kind: Global, Safety: core.Caution,
		Why: "Models fetched with llama-cli -hf; re-downloaded on next run.",
		Paths: func(e *Env) []string {
			return e.envOr("LLAMA_CACHE", filepath.Join(e.XDGCache, "llama.cpp"))
		}},
	{ID: "ai.vllm", Ecosystem: "AI models", Source: "vllm", Name: "vLLM compile cache", Kind: Global, Safety: core.Safe,
		Why:   "torch.compile and CUDA graph caches; rebuilt on next server start.",
		Paths: func(e *Env) []string { return []string{filepath.Join(e.XDGCache, "vllm")} }},

	// ---------------------------------------------------------------- Docker
	{ID: "docker.build-cache", Ecosystem: "Docker", Source: "docker", Name: "Docker build cache", Kind: Provider, Safety: core.Safe,
		Why: "BuildKit layer cache. Next build is slower, nothing else changes."},
	{ID: "docker.images", Ecosystem: "Docker", Source: "docker", Name: "Unused Docker images", Kind: Provider, Safety: core.Caution,
		Why: "Images not used by any container. Pulled again on demand; locally built ones must be rebuilt."},
	{ID: "docker.containers", Ecosystem: "Docker", Source: "docker", Name: "Stopped containers", Kind: Provider, Safety: core.Caution,
		Why: "Stopped containers' writable layers. Anything written inside them (not in a volume) is lost."},
	{ID: "docker.volumes", Ecosystem: "Docker", Source: "docker", Name: "Unused Docker volumes", Kind: Provider, Safety: core.Guarded,
		Why: "Volumes not attached to a container. These are often database data directories; nothing re-creates them."},

	// ---------------------------------------------------------------- Node
	{ID: "node.modules", Ecosystem: "Node", Source: "npm", Name: "node_modules", Kind: Project, Safety: core.Safe,
		Dirs: []string{"node_modules"}, Markers: []string{"package.json"},
		LockFiles:   []string{"package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb", "bun.lock"},
		LockRestore: nodeLocks,
		NoLockNote:  "no lock file: a reinstall may resolve different versions",
		Why:         "Installed dependencies, fully described by package.json and the lock file.",
		Restore:     "npm install"},
	{ID: "node.framework-cache", Ecosystem: "Node", Source: "build", Name: "Framework build cache", Kind: Project, Safety: core.Safe,
		Dirs:    []string{".next", ".nuxt", ".svelte-kit", ".turbo", ".parcel-cache", ".angular", ".docusaurus", ".expo", ".vercel", ".output"},
		Markers: []string{"package.json"},
		Why:     "Framework build output and caches; regenerated by the next dev/build run.",
		Restore: "npm run build"},
	{ID: "node.npm-cache", Ecosystem: "Node", Source: "npm", Name: "npm cache", Kind: Global, Safety: core.Safe,
		Why: "Content-addressed tarball cache; npm refills it as needed.",
		Paths: func(e *Env) []string {
			if e.GOOS == "windows" {
				return []string{e.lad("npm-cache", "_cacache")}
			}
			return []string{e.H(".npm", "_cacache")}
		}},
	{ID: "node.yarn-cache", Ecosystem: "Node", Source: "yarn", Name: "Yarn cache", Kind: Global, Safety: core.Safe,
		Why: "Yarn's global package cache.",
		Paths: func(e *Env) []string {
			return append(e.pick(filepath.Join(e.UserCache, "Yarn"), filepath.Join(e.XDGCache, "yarn"), e.lad("Yarn", "Cache")),
				e.H(".yarn", "berry", "cache"))
		}},
	{ID: "node.pnpm-store", Ecosystem: "Node", Source: "pnpm", Name: "pnpm store", Kind: Global, Safety: core.Caution,
		Why:     "Content-addressed store that node_modules folders hard-link into. Only files no project links to are actually freed.",
		Restore: "pnpm install",
		Paths: func(e *Env) []string {
			return e.envOr("PNPM_STORE_DIR", e.pick(e.H("Library", "pnpm", "store"), e.H(".local", "share", "pnpm", "store"), e.lad("pnpm", "store"))...)
		}},
	{ID: "node.bun-cache", Ecosystem: "Node", Source: "bun", Name: "Bun install cache", Kind: Global, Safety: core.Safe,
		Why:   "Bun's global package cache.",
		Paths: func(e *Env) []string { return []string{e.H(".bun", "install", "cache")} }},
	{ID: "node.playwright", Ecosystem: "Node", Source: "playwright", Name: "Playwright browsers", Kind: Global, Safety: core.Caution,
		Why: "Downloaded Chromium/Firefox/WebKit builds. Tests fail until reinstalled.", Restore: "npx playwright install",
		Paths: func(e *Env) []string {
			return e.envOr("PLAYWRIGHT_BROWSERS_PATH", e.pick(filepath.Join(e.UserCache, "ms-playwright"), filepath.Join(e.XDGCache, "ms-playwright"), e.lad("ms-playwright"))...)
		}},
	{ID: "node.puppeteer", Ecosystem: "Node", Source: "puppeteer", Name: "Puppeteer browsers", Kind: Global, Safety: core.Caution,
		Why: "Downloaded Chrome builds for Puppeteer.", Restore: "npx puppeteer browsers install chrome",
		Paths: func(e *Env) []string { return []string{e.H(".cache", "puppeteer")} }},
	{ID: "node.electron", Ecosystem: "Node", Source: "electron", Name: "Electron download cache", Kind: Global, Safety: core.Safe,
		Why: "Cached Electron release zips.",
		Paths: func(e *Env) []string {
			return e.pick(filepath.Join(e.UserCache, "electron"), filepath.Join(e.XDGCache, "electron"), e.lad("electron", "Cache"))
		}},
	{ID: "node.gyp", Ecosystem: "Node", Source: "node-gyp", Name: "node-gyp headers", Kind: Global, Safety: core.Safe,
		Why: "Node headers for native addon builds; re-fetched automatically.",
		Paths: func(e *Env) []string {
			return e.pick(filepath.Join(e.UserCache, "node-gyp"), filepath.Join(e.XDGCache, "node-gyp"), e.lad("node-gyp", "Cache"))
		}},
	{ID: "node.deno", Ecosystem: "Node", Source: "deno", Name: "Deno cache", Kind: Global, Safety: core.Safe,
		Why: "Deno's module and compile cache.",
		Paths: func(e *Env) []string {
			return e.envOr("DENO_DIR", e.pick(filepath.Join(e.UserCache, "deno"), filepath.Join(e.XDGCache, "deno"), e.lad("deno"))...)
		}},

	// ---------------------------------------------------------------- Python
	{ID: "python.venv", Ecosystem: "Python", Source: "venv", Name: "Virtual environment", Kind: Project, Safety: core.Caution,
		InnerMarker: "pyvenv.cfg",
		LockFiles:   []string{"uv.lock", "poetry.lock", "Pipfile.lock", "requirements.txt", "pyproject.toml", "environment.yml", "setup.py"},
		LockRestore: pyManifests,
		NoLockNote:  "no requirements/pyproject next to it: installed packages may not be reproducible",
		Why:         "A virtualenv (has pyvenv.cfg). Recreated from the project's dependency manifest."},
	{ID: "python.tool-cache", Ecosystem: "Python", Source: "python", Name: "Python tool caches", Kind: Project, Safety: core.Safe,
		Dirs: []string{".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox", ".nox", ".hypothesis"},
		Why:  "Test runner and linter caches; rebuilt on next run."},
	{ID: "python.pip-cache", Ecosystem: "Python", Source: "pip", Name: "pip cache", Kind: Global, Safety: core.Safe,
		Why: "Downloaded wheels and HTTP cache.",
		Paths: func(e *Env) []string {
			return e.envOr("PIP_CACHE_DIR", e.pick(filepath.Join(e.UserCache, "pip"), filepath.Join(e.XDGCache, "pip"), e.lad("pip", "Cache"))...)
		}},
	{ID: "python.uv-cache", Ecosystem: "Python", Source: "uv", Name: "uv cache", Kind: Global, Safety: core.Safe,
		Why: "uv's wheel and source cache. Files hard-linked into live venvs are reported as shared and stay.",
		Paths: func(e *Env) []string {
			return e.envOr("UV_CACHE_DIR", e.pick(filepath.Join(e.XDGCache, "uv"), filepath.Join(e.XDGCache, "uv"), e.lad("uv", "cache"))...)
		}},
	{ID: "python.poetry-cache", Ecosystem: "Python", Source: "poetry", Name: "Poetry cache", Kind: Global, Safety: core.Safe,
		Why: "Poetry's package and artifact caches (not its virtualenvs).",
		Paths: func(e *Env) []string {
			base := e.pick(filepath.Join(e.UserCache, "pypoetry"), filepath.Join(e.XDGCache, "pypoetry"), e.lad("pypoetry", "Cache"))
			var out []string
			for _, b := range base {
				out = append(out, filepath.Join(b, "cache"), filepath.Join(b, "artifacts"))
			}
			return out
		}},
	{ID: "python.poetry-venvs", Ecosystem: "Python", Source: "poetry", Name: "Poetry virtualenvs", Kind: Global, Safety: core.Caution, PerChild: true,
		Why: "Centrally stored Poetry environments; `poetry install` in the project recreates one.", Restore: "poetry install",
		Paths: func(e *Env) []string {
			base := e.pick(filepath.Join(e.UserCache, "pypoetry"), filepath.Join(e.XDGCache, "pypoetry"), e.lad("pypoetry", "Cache"))
			var out []string
			for _, b := range base {
				out = append(out, filepath.Join(b, "virtualenvs"))
			}
			return out
		}},
	{ID: "python.conda-pkgs", Ecosystem: "Python", Source: "conda", Name: "conda package cache", Kind: Global, Safety: core.Caution,
		Why: "Extracted packages and tarballs. Packages hard-linked into environments are reported as shared and stay.",
		Paths: func(e *Env) []string {
			return []string{e.H("miniconda3", "pkgs"), e.H("anaconda3", "pkgs"), e.H("miniforge3", "pkgs"), e.H("mambaforge", "pkgs"), e.H(".conda", "pkgs")}
		},
		Command: func(e *Env) []string {
			if e.has("conda") {
				return []string{"conda", "clean", "--all", "--yes"}
			}
			return nil
		}},

	// ---------------------------------------------------------------- Rust
	{ID: "rust.target", Ecosystem: "Rust", Source: "cargo", Name: "Cargo target dir", Kind: Project, Safety: core.Safe,
		Dirs: []string{"target"}, Markers: []string{"Cargo.toml"},
		Why: "Compiled artifacts; the next cargo build recreates them.", Restore: "cargo build"},
	{ID: "rust.registry", Ecosystem: "Rust", Source: "cargo", Name: "Cargo registry & git cache", Kind: Global, Safety: core.Safe,
		Why: "Downloaded crate sources; Cargo re-fetches what it needs.",
		Paths: func(e *Env) []string {
			home := e.envOr("CARGO_HOME", e.H(".cargo"))[0]
			return []string{filepath.Join(home, "registry", "cache"), filepath.Join(home, "registry", "src"), filepath.Join(home, "git", "checkouts")}
		}},

	// ---------------------------------------------------------------- Go
	{ID: "go.build-cache", Ecosystem: "Go", Source: "go", Name: "Go build cache", Kind: Global, Safety: core.Safe,
		Why: "Compiler and test cache; rebuilt transparently.",
		Paths: func(e *Env) []string {
			return e.envOr("GOCACHE", filepath.Join(e.UserCache, "go-build"))
		},
		Command: func(e *Env) []string {
			if e.has("go") {
				return []string{"go", "clean", "-cache"}
			}
			return nil
		}},
	{ID: "go.mod-cache", Ecosystem: "Go", Source: "go", Name: "Go module cache", Kind: Global, Safety: core.Caution,
		Why: "Downloaded modules (read-only files). Re-downloaded on next build; offline builds break until then.", Restore: "go mod download",
		Paths: func(e *Env) []string {
			if v := e.Getenv("GOMODCACHE"); v != "" {
				return []string{v}
			}
			gopath := e.envOr("GOPATH", e.H("go"))[0]
			return []string{filepath.Join(gopath, "pkg", "mod")}
		},
		Command: func(e *Env) []string {
			if e.has("go") {
				return []string{"go", "clean", "-modcache"}
			}
			return nil
		}},

	// ---------------------------------------------------------------- JVM
	{ID: "jvm.maven-target", Ecosystem: "JVM", Source: "maven", Name: "Maven target dir", Kind: Project, Safety: core.Safe,
		Dirs: []string{"target"}, Markers: []string{"pom.xml"},
		Why: "Maven build output.", Restore: "mvn package"},
	{ID: "jvm.gradle-build", Ecosystem: "JVM", Source: "gradle", Name: "Gradle build output", Kind: Project, Safety: core.Safe,
		Dirs: []string{"build", ".gradle"}, Markers: []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"},
		Why: "Gradle build output and project cache.", Restore: "./gradlew build"},
	{ID: "jvm.gradle-cache", Ecosystem: "JVM", Source: "gradle", Name: "Gradle caches & wrappers", Kind: Global, Safety: core.Safe,
		Why: "Dependency cache and downloaded Gradle distributions.",
		Paths: func(e *Env) []string {
			home := e.envOr("GRADLE_USER_HOME", e.H(".gradle"))[0]
			return []string{filepath.Join(home, "caches"), filepath.Join(home, "wrapper", "dists")}
		}},
	{ID: "jvm.maven-repo", Ecosystem: "JVM", Source: "maven", Name: "Maven local repository", Kind: Global, Safety: core.Caution,
		Why:   "Mostly downloaded artifacts, but it also holds anything you `mvn install`ed locally, which no server can give back.",
		Paths: func(e *Env) []string { return []string{e.H(".m2", "repository")} }},

	// ---------------------------------------------------------------- Xcode / Apple
	{ID: "xcode.derived-data", Ecosystem: "Xcode", Source: "xcode", Name: "DerivedData", Kind: Global, Safety: core.Safe, PerChild: true, OS: []string{"darwin"},
		Why: "Per-project build products and indexes. Xcode rebuilds them on open.", ChildName: stripXcodeHash,
		Paths: func(e *Env) []string { return []string{e.H("Library", "Developer", "Xcode", "DerivedData")} }},
	{ID: "xcode.archives", Ecosystem: "Xcode", Source: "xcode", Name: "Xcode archives", Kind: Global, Safety: core.Guarded, PerChild: true, OS: []string{"darwin"},
		Why:   "Archived app builds and their dSYMs. You need these to symbolicate crash reports from shipped versions.",
		Paths: func(e *Env) []string { return []string{e.H("Library", "Developer", "Xcode", "Archives")} }},
	{ID: "xcode.device-support", Ecosystem: "Xcode", Source: "xcode", Name: "Device support files", Kind: Global, Safety: core.Caution, PerChild: true, OS: []string{"darwin"},
		Why: "Debug symbols copied from each OS version of a connected device. Re-copied (slowly) next time you plug one in.",
		Paths: func(e *Env) []string {
			var out []string
			for _, p := range []string{"iOS", "watchOS", "tvOS", "visionOS"} {
				out = append(out, e.H("Library", "Developer", "Xcode", p+" DeviceSupport"))
			}
			return out
		}},
	{ID: "xcode.caches", Ecosystem: "Xcode", Source: "xcode", Name: "Xcode & simulator caches", Kind: Global, Safety: core.Safe, OS: []string{"darwin"},
		Why: "Xcode's own caches, SwiftUI preview data and simulator dyld caches.",
		Paths: func(e *Env) []string {
			return []string{e.H("Library", "Caches", "com.apple.dt.Xcode"), e.H("Library", "Developer", "CoreSimulator", "Caches"), e.H("Library", "Developer", "Xcode", "UserData", "Previews")}
		}},
	{ID: "xcode.cocoapods-cache", Ecosystem: "Xcode", Source: "cocoapods", Name: "CocoaPods cache", Kind: Global, Safety: core.Safe, OS: []string{"darwin"},
		Why:   "Downloaded pod sources.",
		Paths: func(e *Env) []string { return []string{e.H("Library", "Caches", "CocoaPods")} }},
	{ID: "xcode.pods", Ecosystem: "Xcode", Source: "cocoapods", Name: "Pods directory", Kind: Project, Safety: core.Safe,
		Dirs: []string{"Pods"}, Markers: []string{"Podfile"}, LockFiles: []string{"Podfile.lock"},
		NoLockNote: "no Podfile.lock: pod install may resolve different versions",
		Why:        "Installed pods, described by Podfile and Podfile.lock.", Restore: "pod install"},
	{ID: "xcode.swiftpm-build", Ecosystem: "Xcode", Source: "swiftpm", Name: "SwiftPM .build", Kind: Project, Safety: core.Safe,
		Dirs: []string{".build"}, Markers: []string{"Package.swift"},
		Why: "Swift package build output.", Restore: "swift build"},

	// ---------------------------------------------------------------- Android
	{ID: "android.system-images", Ecosystem: "Android", Source: "sdk", Name: "Android system images", Kind: Global, Safety: core.Caution, PerChild: true,
		Why: "Emulator OS images, re-downloadable from the SDK manager.", Restore: "sdkmanager \"system-images;…\"",
		Paths: func(e *Env) []string {
			if v := e.Getenv("ANDROID_HOME"); v != "" {
				return []string{filepath.Join(v, "system-images")}
			}
			return e.pick(e.H("Library", "Android", "sdk", "system-images"), e.H("Android", "Sdk", "system-images"), e.lad("Android", "Sdk", "system-images"))
		}},
	{ID: "android.avd", Ecosystem: "Android", Source: "avd", Name: "Android virtual devices", Kind: Global, Safety: core.Guarded, PerChild: true,
		Why:   "Emulator disks hold installed apps, accounts and test data.",
		Paths: func(e *Env) []string { return []string{e.H(".android", "avd")} }},
	{ID: "android.dart-build", Ecosystem: "Android", Source: "flutter", Name: "Flutter/Dart build", Kind: Project, Safety: core.Safe,
		Dirs: []string{".dart_tool", "build"}, Markers: []string{"pubspec.yaml"},
		Why: "Flutter build output and tool state.", Restore: "flutter pub get"},

	// ---------------------------------------------------------------- Native / other languages
	{ID: "native.cmake", Ecosystem: "Native builds", Source: "cmake", Name: "CMake build dir", Kind: Project, Safety: core.Safe,
		Dirs: []string{"build", "cmake-build-debug", "cmake-build-release", "out"}, InnerMarker: "CMakeCache.txt",
		Why: "CMake build tree (has CMakeCache.txt); reconfigure to recreate.", Restore: "cmake -B build && cmake --build build"},
	{ID: "native.zig", Ecosystem: "Native builds", Source: "zig", Name: "Zig cache", Kind: Project, Safety: core.Safe,
		Dirs: []string{".zig-cache", "zig-cache", "zig-out"}, Markers: []string{"build.zig"},
		Why: "Zig build cache and output.", Restore: "zig build"},
	{ID: "native.ccache", Ecosystem: "Native builds", Source: "ccache", Name: "ccache / sccache", Kind: Global, Safety: core.Safe,
		Why: "Compiler caches; only cost is a slower next build.",
		Paths: func(e *Env) []string {
			return append(e.pick(filepath.Join(e.UserCache, "ccache"), filepath.Join(e.XDGCache, "ccache"), e.lad("ccache")),
				e.pick(filepath.Join(e.UserCache, "Mozilla.sccache"), filepath.Join(e.XDGCache, "sccache"), e.lad("Mozilla", "sccache"))...)
		}},
	{ID: "native.bazel", Ecosystem: "Native builds", Source: "bazel", Name: "Bazel output base", Kind: Global, Safety: core.Safe, OS: []string{"linux"},
		Why:   "Bazel's output and repository cache.",
		Paths: func(e *Env) []string { return []string{filepath.Join(e.XDGCache, "bazel")} }},
	{ID: "dotnet.bin-obj", Ecosystem: "Other languages", Source: "dotnet", Name: ".NET bin/obj", Kind: Project, Safety: core.Safe,
		Dirs: []string{"bin", "obj"}, Markers: []string{"*.csproj", "*.fsproj", "*.vbproj"},
		Why: ".NET build output.", Restore: "dotnet build"},
	{ID: "elixir.build", Ecosystem: "Other languages", Source: "mix", Name: "Elixir _build/deps", Kind: Project, Safety: core.Safe,
		Dirs: []string{"_build", "deps"}, Markers: []string{"mix.exs"},
		Why: "Compiled BEAM files and fetched deps.", Restore: "mix deps.get && mix compile"},
	{ID: "haskell.build", Ecosystem: "Other languages", Source: "haskell", Name: "Stack/Cabal build", Kind: Project, Safety: core.Safe,
		Dirs: []string{".stack-work", "dist-newstyle"}, Markers: []string{"stack.yaml", "cabal.project", "*.cabal"},
		Why: "Haskell build output.", Restore: "stack build / cabal build"},
	{ID: "php.vendor", Ecosystem: "Other languages", Source: "composer", Name: "Composer vendor", Kind: Project, Safety: core.Caution,
		Dirs: []string{"vendor"}, Markers: []string{"composer.json"}, LockFiles: []string{"composer.lock"},
		NoLockNote: "no composer.lock: reinstall may resolve different versions",
		Why:        "Installed PHP packages. Some projects commit vendor/; devsweep refuses git-tracked paths.", Restore: "composer install"},
	{ID: "terraform.providers", Ecosystem: "Other languages", Source: "terraform", Name: ".terraform", Kind: Project, Safety: core.Caution,
		Dirs: []string{".terraform"}, Markers: []string{"*.tf"},
		Why: "Downloaded providers and modules. `terraform init` restores them; state lives elsewhere.", Restore: "terraform init"},

	// ---------------------------------------------------------------- Tool caches
	{ID: "tools.homebrew", Ecosystem: "Tool caches", Source: "brew", Name: "Homebrew download cache", Kind: Global, Safety: core.Safe, OS: []string{"darwin", "linux"},
		Why: "Downloaded bottles and source tarballs.",
		Paths: func(e *Env) []string {
			return e.envOr("HOMEBREW_CACHE", e.pick(filepath.Join(e.UserCache, "Homebrew"), filepath.Join(e.XDGCache, "Homebrew"), "")...)
		}},
	{ID: "tools.jetbrains", Ecosystem: "Tool caches", Source: "jetbrains", Name: "JetBrains IDE caches", Kind: Global, Safety: core.Safe, PerChild: true,
		Why: "Index and VFS caches per IDE version; old IDE versions' folders are pure waste.",
		Paths: func(e *Env) []string {
			return e.pick(filepath.Join(e.UserCache, "JetBrains"), filepath.Join(e.XDGCache, "JetBrains"), e.lad("JetBrains"))
		}},
	{ID: "tools.composer-cache", Ecosystem: "Tool caches", Source: "composer", Name: "Composer cache", Kind: Global, Safety: core.Safe,
		Why: "Downloaded PHP package archives.",
		Paths: func(e *Env) []string {
			return append(e.pick(filepath.Join(e.UserCache, "composer"), filepath.Join(e.XDGCache, "composer"), e.lad("Composer")), e.H(".composer", "cache"))
		}},
}

func hfHome(e *Env) string {
	if v := e.Getenv("HF_HOME"); v != "" {
		return v
	}
	return filepath.Join(e.XDGCache, "huggingface")
}

// HFHubDir is where the Hugging Face hub cache lives.
func HFHubDir(e *Env) string {
	if v := e.Getenv("HF_HUB_CACHE"); v != "" {
		return v
	}
	if v := e.Getenv("HUGGINGFACE_HUB_CACHE"); v != "" {
		return v
	}
	return filepath.Join(hfHome(e), "hub")
}

// OllamaDirs are the candidate model stores.
func OllamaDirs(e *Env) []string {
	if v := e.Getenv("OLLAMA_MODELS"); v != "" {
		return []string{v}
	}
	dirs := []string{e.H(".ollama", "models")}
	if e.GOOS == "linux" {
		dirs = append(dirs, "/usr/share/ollama/.ollama/models")
	}
	return dirs
}

// stripXcodeHash turns "MyApp-gqbwyxkfjslkdfa" into "MyApp".
func stripXcodeHash(name string) string {
	i := strings.LastIndex(name, "-")
	if i > 0 && len(name)-i-1 >= 20 {
		return name[:i]
	}
	return name
}

// ProjectDirNames is the set of directory names any project rule can match.
func ProjectDirNames() map[string][]*Rule {
	m := map[string][]*Rule{}
	for _, r := range catalog {
		if r.Kind != Project {
			continue
		}
		for _, d := range r.Dirs {
			m[d] = append(m[d], r)
		}
	}
	return m
}
