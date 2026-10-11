import { CodeBlock } from "../components/CodeBlock";
import { Footer, Header } from "../components/Chrome";
import { CopyButton } from "../components/CopyButton";
import { GitHubIcon } from "../components/Icons";
import { Terminal } from "../components/Terminal";
import { DOCS_URL, GO_INSTALL, REPO, doc } from "../site";

const principles = [
  {
    title: "Knows what's safe",
    body: "node_modules only counts next to a package.json, target/ only next to a Cargo.toml. No lockfile, and it's downgraded. Committed to git, and it's refused.",
  },
  {
    title: "Deletes like the tool would",
    body: "Docker gets docker prune, Ollama gets ollama rm, Go gets go clean. When a tool keeps state, devsweep goes through it instead of around it.",
  },
  {
    title: "Counts what you get back",
    body: "Allocated blocks, not apparent size. Files hard-linked from a pnpm store or uv cache are shown as shared, because deleting them frees nothing.",
  },
];

const models: [string, string, string][] = [
  ["ollama", "llama3.1:70b", "39.6 GB"],
  ["ollama", "qwen2.5-coder:32b", "19.9 GB"],
  ["hf", "meta-llama/Llama-3.1-8B-Instruct", "16.1 GB"],
  ["hf", "stabilityai/stable-diffusion-xl-base-1.0", "6.9 GB"],
  ["lmstudio", "mistral-7b-instruct-v0.3.Q4_K_M.gguf", "4.4 GB"],
];

const levels = [
  {
    cls: "safe",
    tag: "● safe",
    body: <>Caches and build output that tools rebuild on their own. Picked by default.</>,
    ex: "DerivedData · target/ · .next · pip cache",
  },
  {
    cls: "caution",
    tag: "▲ caution",
    body: <>Reproducible, but slow or heavy to restore. Opt in with <code>--safety caution</code>.</>,
    ex: "model weights · venvs · Go module cache",
  },
  {
    cls: "guarded",
    tag: "■ guarded",
    body: <>May hold something nothing can recreate. Selected by hand, confirmed by typing <code>delete</code>.</>,
    ex: "Xcode archives · Docker volumes · AVDs",
  },
];

const coverage: [string, string][] = [
  ["AI models", "Ollama, Hugging Face hub and datasets, LM Studio, PyTorch, Whisper, llama.cpp, vLLM"],
  ["Docker", "Build cache, unused images, stopped containers, volumes"],
  ["Node", "node_modules, .next, .turbo, npm, yarn, pnpm, bun, Playwright"],
  ["Python", "venvs, pip, uv, Poetry, conda, pytest and mypy caches"],
  ["Xcode", "DerivedData, device support, archives, Pods, SwiftPM"],
  ["Rust and Go", "Cargo target and registry, Go build and module caches"],
  ["JVM and Android", "Gradle, Maven, system images, emulators"],
  ["Everything else", "CMake, Zig, .NET, Elixir, Haskell, Terraform, Homebrew, JetBrains"],
];

export function Home() {
  return (
    <>
      <a className="skip" href="#main">Skip to content</a>
      <Header page="home" />

      <main id="main">
        <section className="wrap hero">
          <p className="eyebrow">A disk cleaner for developers</p>
          <h1>Where did 200&nbsp;GB go?</h1>
          <p className="lede">
            devsweep finds the space your tooling quietly eats — node_modules, Docker layers, venvs, Xcode caches and
            local AI models — and removes it with rules that know what is safe.
          </p>
          <div className="cta">
            <a className="btn btn-primary" href={doc("install")}>Get started</a>
            <a className="btn" href={REPO}>
              <GitHubIcon size={16} /> View on GitHub
            </a>
          </div>
          <div className="install">
            <code>
              <span className="prompt">$ </span>
              {GO_INSTALL}
            </code>
            <CopyButton text={GO_INSTALL} label="Copy install command" />
          </div>
          <p className="fine">
            macOS · Linux · Windows. One binary, no runtime.
          </p>
        </section>

        <section className="wrap" style={{ paddingBottom: "6rem" }} aria-label="The devsweep TUI">
          <Terminal />
        </section>

        <section className="band">
          <div className="wrap three">
            {principles.map((p, i) => (
              <div key={p.title}>
                <span className="num">{String(i + 1).padStart(2, "0")}</span>
                <h2>{p.title}</h2>
                <p>{p.body}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="band alt">
          <div className="wrap split-2">
            <div>
              <p className="eyebrow">Local AI models</p>
              <h2 className="big">Models are the new node_modules.</h2>
              <p>
                A few pulls and a couple of Hugging Face checkpoints, and 100 GB is gone. devsweep reads Ollama's
                manifests and the hub cache directly, so every model shows up by name, with when it was pulled and what
                removing it actually frees — tags that share weights are counted once.
              </p>
              <a href={doc("ai-models")} style={{ fontWeight: 500 }}>How model stores are handled →</a>
            </div>
            <div className="list-card">
              {models.map(([src, name, size]) => (
                <div className="li" key={name}>
                  <span>{src}</span>
                  <span>{name}</span>
                  <span>{size}</span>
                </div>
              ))}
              <p className="foot">Ollama, Hugging Face, LM Studio, PyTorch hub, Whisper, llama.cpp, vLLM</p>
            </div>
          </div>
        </section>

        <section className="band" id="safety">
          <div className="wrap">
            <div className="stack" style={{ maxWidth: "40rem" }}>
              <h2 className="big">Three levels. Nothing irreplaceable by accident.</h2>
              <p className="body">Every item gets a level, and the level decides what's allowed to select it.</p>
            </div>
            <div className="levels">
              {levels.map((l) => (
                <div className="level" key={l.cls}>
                  <span className={`tag ${l.cls}`}>{l.tag}</span>
                  <p>{l.body}</p>
                  <p className="ex">{l.ex}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="band">
          <div className="wrap split-2">
            <div>
              <h2 className="big">Scriptable when you want it.</h2>
              <p>The same rules drive a plain CLI with JSON output, dry runs and filters by ecosystem, age and size.</p>
              <a href={doc("cli")} style={{ fontWeight: 500 }}>CLI reference →</a>
            </div>
            <CodeBlock
              code={`$ devsweep scan --eco ai,docker --min-size 1GB
$ devsweep clean --older-than 30d --dry-run
would free 37.2 GB across 38 items (dry run)
$ devsweep clean --eco node,rust --older-than 60d
$ devsweep scan --json | jq '.[].name'`}
            />
          </div>
        </section>

        <section className="band">
          <div className="wrap">
            <h2 className="small-h">What it covers</h2>
            <div className="covers">
              {coverage.map(([name, items]) => (
                <div key={name}>
                  <h3>{name}</h3>
                  <p>{items}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="band alt">
          <div className="wrap closing">
            <h2 className="big">Get your disk back in a minute.</h2>
            <p>Scanning is read-only. Look first, then decide.</p>
            <div className="cta">
              <a className="btn btn-primary" href={doc("quickstart")}>Get started</a>
              <a className="btn" href={DOCS_URL}>Read the docs</a>
            </div>
          </div>
        </section>
      </main>

      <Footer>
        <a href={DOCS_URL}>Docs</a>
      </Footer>
    </>
  );
}
