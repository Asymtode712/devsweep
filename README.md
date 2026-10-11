<div align="center">

# devsweep

**Where did 200 GB go on my dev machine?**

A disk cleaner for developers. devsweep finds the space your tooling quietly eats (`node_modules`, Docker layers, virtualenvs, Xcode caches, local AI models) and removes it with per-ecosystem rules that know what's safe.

[![CI](https://github.com/Asymtode712/devsweep/actions/workflows/ci.yml/badge.svg)](https://github.com/Asymtode712/devsweep/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Asymtode712/devsweep.svg)](https://pkg.go.dev/github.com/Asymtode712/devsweep)
[![Go version](https://img.shields.io/github/go-mod/go-version/Asymtode712/devsweep)](go.mod)

[Website](https://asymtode712.github.io/devsweep/) · [Documentation](https://asymtode712.github.io/devsweep/docs.html) · [Report an issue](https://github.com/Asymtode712/devsweep/issues)

</div>

---

```text
$ devsweep scan --min-size 1GB
scanned ~ · 9 items · 1.1s

  ECOSYSTEM       SOURCE      ITEM                                       FREES     AGE
▲ AI models       ollama      llama3.1:70b                             39.6 GB     5mo
▲ Docker          docker      unused images (14)                       22.3 GB       —
▲ AI models       hf          meta-llama/Llama-3.1-8B-Instruct         16.1 GB     3mo
● Docker          docker      build cache                              12.9 GB       —
● Xcode           xcode       DerivedData/Ledger                        8.4 GB     2mo
■ Xcode           xcode       Archives/2025-11-02                       6.2 GB    11mo
● Rust            cargo       ~/code/tracer/target                      5.7 GB     6mo
▲ Python          venv        ~/code/rag-eval/.venv                     4.8 GB     4mo
● Node            npm         ~/code/dashboard/node_modules             1.9 GB     8mo

● safe 28.9 GB   ▲ caution 82.8 GB   ■ guarded 6.2 GB   total 117.9 GB
```

## Why devsweep

General-purpose cleaners see a 2 GB folder. devsweep sees a `node_modules` next to a `package-lock.json`, knows `npm ci` restores it, and tells you so before deleting it.

- **Knows what's safe.** `node_modules` only counts next to a `package.json`, and `target/` only next to a `Cargo.toml` or `pom.xml`. With no lockfile, the item is downgraded. If it's committed to git, it's refused.
- **Deletes like the tool would.** Docker gets `docker prune`, Ollama gets `ollama rm`, and Go gets `go clean`. When a tool keeps its own state, devsweep goes through it instead of around it.
- **Counts what you actually get back.** It reports allocated blocks, not apparent size. Files hard-linked from a pnpm store, uv cache or conda pkgs are shown as *shared*, because deleting them frees nothing.
- **Understands local AI models.** It reads Ollama manifests and the Hugging Face hub cache directly, so models show up by name. Weights shared between tags are counted once.
- **One binary, two interfaces.** A TUI for browsing and choosing, and a scriptable CLI with JSON output. It runs on macOS, Linux and Windows with no runtime.

## Installation

**Go 1.23+**

```sh
go install github.com/Asymtode712/devsweep@latest
```

**From source**

```sh
git clone https://github.com/Asymtode712/devsweep.git
cd devsweep && make build
```

## Quick start

```sh
devsweep                                   # interactive TUI over your home directory
devsweep scan --min-size 500MB             # read-only report, largest first
devsweep clean --older-than 30d --dry-run  # what a safe cleanup would remove
devsweep clean --older-than 30d            # do it (asks first)
```

In the TUI, press <kbd>s</kbd> to pick every *safe* item untouched for 30 days, review with <kbd>d</kbd>, and confirm.

| Key | Action | Key | Action |
| --- | --- | --- | --- |
| `↑` `↓` `tab` | move, switch panes | `/` `o` | filter, cycle sort |
| `space` | select item | `d` | review and delete |
| `a` | select all in view (never guarded) | `t` | toggle move-to-Trash |
| `s` | smart-pick safe + stale | `r` `q` | rescan, quit |

## Usage

```text
devsweep [paths...]          interactive TUI (default root: your home directory)
devsweep scan [paths...]     list what can be reclaimed
devsweep clean [paths...]    delete matching items (asks first)
devsweep rules               show every rule and why it's considered safe
devsweep version
```

| Flag | Applies to | Description |
| --- | --- | --- |
| `--eco LIST` | scan, clean | Ecosystems or sources: `ai`, `docker`, `node`, `python`, `xcode`, `rust`, `go`, `jvm`, `ollama`, `hf`, … |
| `--safety LEVEL` | scan, clean | Highest level to include: `safe` · `caution` · `guarded` (clean defaults to `safe`) |
| `--older-than AGE` | scan, clean | Only items untouched for `30d`, `2w`, `6mo`, `1y` |
| `--min-size SIZE` | scan, clean | Only items freeing at least `500MB`, `2GB` |
| `--no-global` / `--no-projects` | scan, clean | Skip global caches, or skip the project walk |
| `--json` | scan, rules | Machine-readable output |
| `--dry-run` | clean | Show what would be deleted; touch nothing |
| `--yes` | clean | Don't prompt (for scripts and cron) |
| `--trash` | clean | Move to Trash instead of deleting (macOS, Linux) |

Full reference with recipes: **[documentation](https://asymtode712.github.io/devsweep/docs.html#cli)**.

## Safety model

Every item gets one of three levels, and the level decides what can select it.

| | Level | Meaning | Selected by |
| --- | --- | --- | --- |
| ● | **safe** | Cache or build output that tools recreate on their own | `clean` by default, `s`, `a` |
| ▲ | **caution** | Reproducible, but slow or heavy to restore (model weights, venvs, module caches) | `--safety caution`, `a`, `space` |
| ■ | **guarded** | May hold something nothing can recreate (Xcode archives, Docker volumes, AVDs) | only by hand, after typing `delete` |

Right before anything is removed, each item is checked again:

- The project manifest must still be there.
- The path must not be tracked in git.
- The path must not be (or have become) a symlink.
- The path must not be your home directory, a cache root, or a top-level directory.

Every deletion is appended to `~/.local/state/devsweep/history.jsonl` (`%LOCALAPPDATA%\devsweep` on Windows), together with the command that restores it.

## What it covers

| Ecosystem | Items |
| --- | --- |
| **AI models** | Ollama, Hugging Face hub, datasets and Xet caches, LM Studio, PyTorch hub, Whisper, llama.cpp, vLLM |
| **Docker** | Build cache, unused images, stopped containers, unused volumes |
| **Node** | `node_modules`, `.next` / `.nuxt` / `.turbo` / `.svelte-kit` and friends, npm, yarn, pnpm, bun, Playwright, Puppeteer, Electron, Deno |
| **Python** | Virtualenvs (by `pyvenv.cfg`), pytest / mypy / ruff / tox caches, pip, uv, Poetry, conda |
| **Xcode** | DerivedData, device support, archives, simulator caches, CocoaPods, SwiftPM `.build` |
| **Rust · Go · JVM** | Cargo `target` and registry, Go build and module caches, Maven, Gradle |
| **Android** | System images, AVDs, Flutter / Dart build output |
| **Other** | CMake, Zig, .NET, Elixir, Haskell, Composer, Terraform, ccache / sccache, Bazel, Homebrew, JetBrains |

Run `devsweep rules` for the full catalog, or browse it in the [docs](https://asymtode712.github.io/devsweep/docs.html#rules).

## How it works

1. **Discover.** A parallel walk of your project roots matches artifact folders against their manifests, while known per-user cache locations and tool metadata (Docker, Ollama, Hugging Face) are read concurrently. Symlinks are never followed. On macOS, privacy-protected folders are skipped unless passed explicitly.
2. **Measure.** Allocated blocks are summed per item, and hardlinks are tracked by inode. That's how bytes shared with something outside the item are reported separately from what deleting it frees.
3. **Clean.** Each item is re-validated, then removed through the tool's own command where one exists, or deleted (or moved to the Trash) and logged.

## Known limitations

- APFS clones (uv's default on macOS) share blocks invisibly, so a cloned venv may free less than shown.
- Docker Desktop frees space inside its VM. The host gets it back as the VM disk is trimmed.
- Move-to-Trash isn't available on Windows, and Windows sizes don't account for hardlinks.

## Contributing

Issues and pull requests are welcome. Most new ecosystems are a single rule in [`internal/rules/rules.go`](internal/rules/rules.go).

```sh
make build    # build ./devsweep
make test     # go vet + unit tests
make dist     # cross-compiled binaries in dist/
make site     # regenerate the rule catalog and build the website (site/)
```

The website is React + TypeScript (Vite) in [`site/`](site/). It's deployed to GitHub Pages by [`.github/workflows/pages.yml`](.github/workflows/pages.yml).
