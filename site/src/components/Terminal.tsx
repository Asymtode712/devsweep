type Safety = "safe" | "caution";

type Row = { on: boolean; safety: Safety; src: string; name: string; size: string; age: string };

// Example data illustrating the TUI; not from a real machine.
const ecosystems: [string, string][] = [
  ["AI models", "89.2 GB"],
  ["Docker", "38.6 GB"],
  ["Node", "29.3 GB"],
  ["Xcode", "21.8 GB"],
  ["Python", "11.2 GB"],
  ["Rust", "7.9 GB"],
];

const rows: Row[] = [
  { on: true, safety: "caution", src: "ollama", name: "llama3.1:70b", size: "39.6 GB", age: "5mo" },
  { on: false, safety: "caution", src: "ollama", name: "qwen2.5-coder:32b", size: "19.9 GB", age: "2d" },
  { on: true, safety: "caution", src: "hf", name: "meta-llama/Llama-3.1-8B-Instruct", size: "16.1 GB", age: "3mo" },
  { on: true, safety: "caution", src: "hf", name: "stabilityai/stable-diffusion-xl-base-1.0", size: "6.9 GB", age: "7mo" },
  { on: false, safety: "caution", src: "lmstudio", name: "mistral-7b-instruct-v0.3.Q4_K_M", size: "4.4 GB", age: "1mo" },
  { on: true, safety: "safe", src: "torch", name: "torch hub checkpoints", size: "812 MB", age: "6mo" },
];

const glyph: Record<Safety, string> = { safe: "●", caution: "▲" };

/** A faithful HTML rendering of the devsweep TUI, used as the hero visual. */
export function Terminal() {
  return (
    <figure style={{ margin: 0 }}>
      <div
        className="term"
        role="img"
        aria-label="devsweep's terminal interface listing local AI models by size, with llama3.1:70b selected and its removal command shown"
      >
        <div className="term-inner" aria-hidden="true">
          <div className="term-top">
            <span>
              <span className="badge">devsweep</span>
              <span className="dim">  scanned ~ in 1.0s · 214 items</span>
            </span>
            <span>
              <span className="dim">reclaimable </span>263 GB<span className="dim">   selected </span>
              <span className="acc">64.6 GB</span>
            </span>
          </div>
          <div className="split">
            <div className="col">
              <span className="dim">ECOSYSTEMS</span>
              {ecosystems.map(([name, size], i) => (
                <span key={name} className={i === 0 ? "eco cur" : "eco"}>
                  <span>{(i === 0 ? "▸ " : "  ") + name}</span>
                  <span>{size}</span>
                </span>
              ))}
            </div>
            <div className="col">
              <span className="dim">AI models</span>
              {rows.map((r, i) => (
                <div key={r.name} className={i === 0 ? "row cur" : "row"}>
                  <span className={r.on ? "acc" : "t-off"}>{r.on ? "[x]" : "[ ]"}</span>
                  <span className={`t-${r.safety}`}>{glyph[r.safety]}</span>
                  <span className="dim">{r.src}</span>
                  <span>{r.name}</span>
                  <span>{r.size}</span>
                  <span className="dim">{r.age}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="kv">
            <span className="dim">action</span>
            <span>
              runs <span className="acc">ollama rm llama3.1:70b</span> — shared blobs are never removed directly
            </span>
            <span className="dim">restore</span>
            <span>ollama pull llama3.1:70b</span>
          </div>
        </div>
      </div>
      <figcaption className="term-caption">
        The TUI. Ecosystems on the left, items by size on the right, and the rule for the focused item underneath.
      </figcaption>
    </figure>
  );
}
