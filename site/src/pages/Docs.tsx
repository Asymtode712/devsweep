import type { ReactNode } from "react";
import { CodeBlock } from "../components/CodeBlock";
import { Footer, Header } from "../components/Chrome";
import { InfoIcon } from "../components/Icons";
import { RuleCatalog, rules } from "../docs/RuleCatalog";
import { Sidebar, type NavGroup } from "../docs/Sidebar";
import { GO_INSTALL, HOME_URL, ISSUES, REPO } from "../site";

const nav: NavGroup[] = [
  { title: "Get started", links: [{ id: "intro", label: "Introduction" }, { id: "install", label: "Install" }, { id: "quickstart", label: "Quick start" }] },
  { title: "Using devsweep", links: [{ id: "tui", label: "The TUI" }, { id: "cli", label: "CLI reference" }, { id: "recipes", label: "Recipes" }] },
  { title: "Safety", links: [{ id: "levels", label: "Safety levels" }, { id: "rails", label: "Deletion rails" }, { id: "history", label: "History log" }] },
  { title: "Reference", links: [{ id: "rules", label: "Rule catalog" }, { id: "ai-models", label: "AI model stores" }, { id: "limits", label: "Limits and FAQ" }] },
];

function Section({ id, kicker, title, h1, children }: { id: string; kicker?: string; title: string; h1?: boolean; children: ReactNode }) {
  return (
    <section id={id}>
      {kicker && <span className="section-kicker">{kicker}</span>}
      {h1 ? <h1>{title}</h1> : <h2>{title}</h2>}
      {children}
    </section>
  );
}

function Table({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  return (
    <div className="table">
      <table>
        <thead>
          <tr>{head.map((h) => <th key={h}>{h}</th>)}</tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>{r.map((c, j) => <td key={j}>{c}</td>)}</tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Note({ children }: { children: ReactNode }) {
  return (
    <div className="note">
      <InfoIcon />
      <span>{children}</span>
    </div>
  );
}

const C = ({ children }: { children: ReactNode }) => <code>{children}</code>;
const K = ({ children }: { children: ReactNode }) => <kbd>{children}</kbd>;

export function Docs() {
  return (
    <>
      <a className="skip" href="#content">Skip to content</a>
      <Header page="docs" />
      <div className="docs">
        <Sidebar groups={nav} />
        <main className="content" id="content">
          <Section id="intro" kicker="Get started" title="devsweep" h1>
            <p className="lead">
              devsweep finds and reclaims disk space used by developer tooling: dependencies, build output, package
              caches, Docker, Xcode, and local AI models. It ships as one binary with a TUI for browsing and a CLI for
              scripting, and both use the same rules.
            </p>
            <p>
              A general-purpose cleaner sees a 2 GB folder. devsweep sees a <C>node_modules</C> next to a{" "}
              <C>package-lock.json</C>, knows <C>npm ci</C> restores it, and tells you so before deleting it. Every
              item carries the rule that found it, a <a href="#levels">safety level</a>, and the command to get it back.
            </p>
          </Section>

          <Section id="install" title="Install">
            <h3>With Go</h3>
            <p>Needs Go 1.23 or newer. The binary lands in <C>$(go env GOPATH)/bin</C>.</p>
            <CodeBlock copy code={`$ ${GO_INSTALL}\n$ devsweep version`} />
            <h3>From source</h3>
            <CodeBlock copy code={`$ git clone ${REPO}.git\n$ cd devsweep\n$ make build`} />
          </Section>

          <Section id="quickstart" title="Quick start">
            <p>Scanning is read-only, so start there. This lists everything over 500 MB, largest first:</p>
            <CodeBlock code="$ devsweep scan --min-size 500MB" />
            <p>
              When you're ready to clean, open the TUI, browse by ecosystem, and press <K>s</K> to pick every{" "}
              <em>safe</em> item that hasn't been touched in 30 days:
            </p>
            <CodeBlock code="$ devsweep" />
            <p>Or do the same from the CLI, checking with a dry run first:</p>
            <CodeBlock code={`$ devsweep clean --older-than 30d --dry-run\n$ devsweep clean --older-than 30d`} />
            <Note>
              On macOS a default scan skips <C>~/Desktop</C>, <C>~/Documents</C> and <C>~/Downloads</C>. Reading them
              triggers a privacy prompt that blocks until answered. Pass them as paths to include them:{" "}
              <C>devsweep ~/Documents</C>.
            </Note>
          </Section>

          <Section id="tui" kicker="Using devsweep" title="The TUI">
            <p>
              Run <C>devsweep</C> with no command, optionally followed by project roots. Ecosystems sit on the left,
              sized and sorted. Items for the selected ecosystem are on the right. Below them, the focused item's rule is
              explained: its path, why it's considered safe, what deleting it frees, exactly what action runs, and how
              to restore it.
            </p>
            <Table
              head={["Key", "Action"]}
              rows={[
                [<><K>↑</K> <K>↓</K> <K>tab</K></>, "Move; switch between the ecosystem list and the item list"],
                [<K>space</K>, "Select or deselect the focused item"],
                [<K>a</K>, "Select everything in view, except guarded items"],
                [<K>s</K>, "Smart-pick: every safe item untouched for 30+ days, across all ecosystems"],
                [<K>c</K>, "Clear the selection"],
                [<><K>/</K> <K>o</K></>, "Filter by name or path; cycle sort between size, age and name"],
                [<K>d</K>, "Review the selection and delete"],
                [<K>t</K>, "Toggle move-to-Trash mode"],
                [<><K>r</K> <K>q</K></>, "Rescan; quit"],
              ]}
            />
            <p>
              Pressing <K>d</K> opens a review screen. It groups the selection into native commands and directories,
              lists restore commands for anything that isn't safe, and shows the deletion mode. If any guarded item is
              selected, you have to type <C>delete</C> before the button activates. Ages shown in the accent color mean
              the item was used in the last week, which is worth a second look.
            </p>
          </Section>

          <Section id="cli" title="CLI reference">
            <p>Every command shares one scanner and one rule catalog, so the TUI and the CLI always agree on what's safe.</p>
            <h3>Commands</h3>
            <Table
              head={["Command", "What it does"]}
              rows={[
                [<C>devsweep [paths…]</C>, <>Open the TUI. Scans your home directory unless you pass roots. When output isn't a terminal, behaves like <C>scan</C>.</>],
                [<C>devsweep scan [paths…]</C>, "List everything reclaimable, largest first, with totals per safety level."],
                [<C>devsweep clean [paths…]</C>, <>Delete what matches the filters. Prints the plan and asks first unless <C>--yes</C>.</>],
                [<C>devsweep rules</C>, "Print every rule, the paths or markers it checks, and why it's safe."],
                [<C>devsweep version</C>, "Print the version."],
              ]}
            />
            <h3>Filters</h3>
            <p>
              Filters work the same for <C>scan</C> and <C>clean</C>. <C>clean</C> only includes safe items unless you
              raise <C>--safety</C>.
            </p>
            <Table
              head={["Flag", "Meaning"]}
              rows={[
                [<C>--eco LIST</C>, <>Comma-separated ecosystems or sources: <C>ai</C>, <C>docker</C>, <C>node</C>, <C>python</C>, <C>xcode</C>, <C>rust</C>, <C>go</C>, <C>jvm</C>, or a source like <C>ollama</C>, <C>hf</C>, <C>pnpm</C>.</>],
                [<C>--safety LEVEL</C>, <>Highest level to include: <C>safe</C>, <C>caution</C> or <C>guarded</C>. Defaults to guarded for <C>scan</C> and safe for <C>clean</C>.</>],
                [<C>--older-than AGE</C>, <>Only items untouched for at least AGE: <C>30d</C>, <C>2w</C>, <C>6mo</C>, <C>1y</C>.</>],
                [<C>--min-size SIZE</C>, <>Only items that free at least SIZE: <C>500MB</C>, <C>2GB</C>. Decimal units.</>],
                [<C>--no-global</C>, "Only walk project folders; skip global caches, models and Docker."],
                [<C>--no-projects</C>, "Only global caches, models and Docker; skip the project walk."],
                [<C>--depth N</C>, "Maximum directory depth for the project walk. Default 12."],
              ]}
            />
            <h3>Output and cleaning</h3>
            <Table
              head={["Flag", "Meaning"]}
              rows={[
                [<C>--json</C>, <>Machine-readable output for <C>scan</C> and <C>rules</C>.</>],
                [<C>--dry-run</C>, <>Show what <C>clean</C> would delete and how much it would free. Touch nothing.</>],
                [<C>--yes</C>, <>Don't prompt. Guarded items still need <C>--safety guarded</C> to be included at all.</>],
                [<C>--trash</C>, "Move to the Trash instead of deleting. Frees nothing until the Trash is emptied. Not available on Windows."],
              ]}
            />
            <p>
              <C>clean</C> refuses to run without a terminal unless you pass <C>--yes</C> or <C>--dry-run</C>, so a
              stray pipe can't delete anything. It exits with status 1 if any item failed.
            </p>
          </Section>

          <Section id="recipes" title="Recipes">
            <h3>Clear stale build output in your code folder</h3>
            <CodeBlock code="$ devsweep clean ~/code --eco node,rust --older-than 60d --dry-run" />
            <h3>See what your local models cost</h3>
            <CodeBlock code="$ devsweep scan --no-projects --eco ai" />
            <h3>Remove old models, including slow-to-restore ones</h3>
            <CodeBlock code="$ devsweep clean --eco ollama,hf --safety caution --older-than 3mo" />
            <h3>Weekly cron job for pure caches</h3>
            <CodeBlock code={`# safe items only, untouched for a month, no prompt\n$ devsweep clean --no-projects --older-than 30d --yes`} />
            <h3>Pipe into other tools</h3>
            <CodeBlock code={`$ devsweep scan --json | jq -r '.[] | select(.safety == "safe") | "\\(.frees)\\t\\(.name)"'`} />
          </Section>

          <Section id="levels" kicker="Safety" title="Safety levels">
            <p>Every item gets one of three levels. The level decides what is allowed to select it.</p>
            <Table
              head={["Level", "Meaning", "Selected by"]}
              rows={[
                [<span className="safe mono">● safe</span>, "Pure cache or build output that tools recreate on their own.", <><C>clean</C> by default, <K>s</K>, <K>a</K></>],
                [<span className="caution mono">▲ caution</span>, "Reproducible, but slow or bandwidth-heavy to restore: model weights, venvs, module caches.", <><C>--safety caution</C>, <K>a</K>, <K>space</K></>],
                [<span className="guarded mono">■ guarded</span>, "May hold something nothing can recreate: Xcode archives, Docker volumes, emulator disks, venvs with no manifest.", <>Only by hand, after typing <C>delete</C></>],
              ]}
            />
            <h3>Levels can drop</h3>
            <p>
              A rule's level is a starting point. If a project artifact has no lockfile next to it, it drops one level:{" "}
              <C>node_modules</C> without a lockfile becomes caution, because a reinstall may resolve different versions.
              A virtualenv with no <C>requirements.txt</C>, <C>pyproject.toml</C>, <C>uv.lock</C> or similar becomes
              guarded, because its packages may not be reproducible.
            </p>
          </Section>

          <Section id="rails" title="Deletion rails">
            <p>
              A scan can be minutes old by the time you delete. Every item is checked again immediately before anything
              is removed, and is skipped and reported if any check fails.
            </p>
            <ul>
              <li><strong>Its manifest is still there.</strong> A project artifact must still sit next to the file that made it match: <C>package.json</C>, <C>Cargo.toml</C>, <C>pyvenv.cfg</C>, <C>CMakeCache.txt</C>.</li>
              <li><strong>It isn't tracked in git.</strong> If <C>git ls-files</C> knows about it, someone committed it, and devsweep won't delete committed files.</li>
              <li><strong>It isn't a symlink.</strong> Symlinks are never followed during a scan. A path that became one since is refused.</li>
              <li><strong>It isn't a protected root.</strong> Your home directory, cache roots like <C>~/.cache</C> or <C>~/Library</C>, and top-level paths are refused outright.</li>
              <li><strong>The tool goes first.</strong> When a tool keeps its own state, devsweep runs its command: <C>docker … prune</C>, <C>ollama rm</C>, <C>go clean</C>, <C>conda clean</C>. If that command fails, it falls back to removing the exact files the rule identified.</li>
            </ul>
            <p>Read-only trees, such as Go's module cache, are made writable and retried, so a cleanup doesn't stop halfway.</p>
          </Section>

          <Section id="history" title="History log">
            <p>Every deletion is appended to a JSON Lines log with what was removed, how, how much it freed, and the command to restore it.</p>
            <Table
              head={["Platform", "Location"]}
              rows={[
                ["macOS, Linux", <><C>~/.local/state/devsweep/history.jsonl</C> (or <C>$XDG_STATE_HOME/devsweep</C>)</>],
                ["Windows", <C>%LOCALAPPDATA%\devsweep\history.jsonl</C>],
              ]}
            />
            <CodeBlock
              code={`$ tail -n 1 ~/.local/state/devsweep/history.jsonl | jq
{
  "command": null,
  "method": "deleted",
  "name": "~/code/tracer/target",
  "paths": ["/Users/you/code/tracer/target"],
  "freed": 5700000000,
  "restore": "cd ~/code/tracer && cargo build",
  "rule": "rust.target",
  "time": "2026-10-11T14:06:00+02:00"
}`}
            />
          </Section>

          <Section id="rules" kicker="Reference" title="Rule catalog">
            <p>
              {rules.length} rules, generated from <C>devsweep rules --json</C>. Project rules match a folder name next
              to a marker file. Global rules check fixed per-user locations, shown here as resolved on macOS. Provider
              rules read the tool's own metadata.
            </p>
            <RuleCatalog />
          </Section>

          <Section id="ai-models" title="AI model stores">
            <p>Model caches are large and have their own structure, so devsweep reads them through their metadata instead of treating them as plain folders.</p>
            <h3>Ollama</h3>
            <p>
              Ollama stores weights as content-addressed blobs, and tags often share them: <C>llama3:latest</C> and{" "}
              <C>llama3:8b</C> can be the same file. devsweep reads every manifest, counts references to each blob, and
              reports a model's <em>freeable</em> size as only the blobs nothing else uses. Removal runs <C>ollama rm</C>.
              If Ollama isn't installed or its server isn't running, devsweep deletes the manifest itself, then removes
              any blob no remaining manifest references. Partial downloads are left alone.
            </p>
            <p>Location: <C>~/.ollama/models</C>, <C>$OLLAMA_MODELS</C>, or <C>/usr/share/ollama/.ollama/models</C> on Linux.</p>
            <h3>Hugging Face</h3>
            <p>
              Each <C>models--org--name</C> folder in the hub cache becomes one item, named as the repo is. Snapshots are
              symlinks into <C>blobs/</C>, and since symlinks are never followed, each file counts once. Datasets and
              Spaces are listed the same way, alongside the separate <C>datasets</C> Arrow cache and the Xet chunk cache.
            </p>
            <p>Location: <C>$HF_HUB_CACHE</C>, <C>$HF_HOME/hub</C>, or <C>~/.cache/huggingface/hub</C>.</p>
            <h3>Others</h3>
            <p>LM Studio models are listed per model (<C>~/.lmstudio/models</C>). PyTorch hub, Whisper, llama.cpp and vLLM caches are listed as one item each.</p>
          </Section>

          <Section id="limits" title="Limits and FAQ">
            <h3>Why does an item free less than its size?</h3>
            <p>
              Some of its files are hard-linked from somewhere else: the pnpm store, uv's cache, or conda's package
              cache. Deleting one link frees nothing while another remains, so those bytes are shown as shared and
              excluded from totals.
            </p>
            <h3>Why do the numbers differ from du?</h3>
            <p>devsweep counts allocated blocks and counts hard-linked files once, the way the filesystem does. It uses decimal units, like Finder and Docker.</p>
            <h3>Known limits</h3>
            <ul>
              <li>APFS clones, which uv uses on macOS by default, share blocks invisibly. A venv cloned from the uv cache may free less than shown.</li>
              <li>Docker Desktop frees space inside its virtual machine. The host gets it back as the VM disk is trimmed.</li>
              <li>Move-to-Trash renames into the Trash on the same volume, so it frees nothing until emptied. It isn't supported on Windows.</li>
              <li>When several Ollama tags share weights, deleting them together frees more than the per-tag numbers show.</li>
              <li>Windows uses apparent file sizes and doesn't detect hard links.</li>
            </ul>
            <h3>Found a cache devsweep misses?</h3>
            <p>
              <a href={ISSUES}>Open an issue</a> with the tool, the path, and how it's restored. Most new ecosystems are a
              single rule.
            </p>
            <div className="pager">
              <a href="#intro">
                <span className="fine block">Back to</span>
                <span className="strong">↑ Introduction</span>
              </a>
              <a href={HOME_URL} style={{ textAlign: "right" }}>
                <span className="fine block">Home</span>
                <span className="strong">devsweep →</span>
              </a>
            </div>
          </Section>
        </main>
      </div>
      <Footer wide>
        <a href={HOME_URL}>Home</a>
      </Footer>
    </>
  );
}
