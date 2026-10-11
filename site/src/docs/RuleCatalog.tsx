import { useMemo, useState } from "react";
import rulesData from "../data/rules.json";

type Level = "safe" | "caution" | "guarded";

/** Shape of `devsweep rules --json`. */
export type Rule = {
  ID: string;
  Ecosystem: string;
  Name: string;
  Kind: "project" | "global" | "provider";
  Safety: Level;
  Why: string;
  Restore: string;
  Dirs?: string[];
  Markers?: string[];
  Paths?: string[];
};

export const rules = rulesData as Rule[];

const glyph: Record<Level, string> = { safe: "●", caution: "▲", guarded: "■" };
const filters: Array<"all" | Level> = ["all", "safe", "caution", "guarded"];

function matchText(r: Rule): string {
  if (r.Kind === "project") {
    const dirs = (r.Dirs ?? []).join(", ");
    return r.Markers?.length ? `${dirs} next to ${r.Markers.join(" | ")}` : dirs;
  }
  if (r.Kind === "global") return (r.Paths ?? []).join(", ");
  return "read from the tool's own metadata";
}

export function RuleCatalog() {
  const [level, setLevel] = useState<"all" | Level>("all");

  const groups = useMemo(() => {
    const out: { eco: string; rules: Rule[] }[] = [];
    for (const r of rules) {
      if (level !== "all" && r.Safety !== level) continue;
      const last = out[out.length - 1];
      if (last && last.eco === r.Ecosystem) last.rules.push(r);
      else out.push({ eco: r.Ecosystem, rules: [r] });
    }
    return out;
  }, [level]);

  return (
    <>
      <div className="rules-tools" role="group" aria-label="Filter rules by level">
        {filters.map((f) => (
          <button
            key={f}
            className="chip"
            type="button"
            aria-pressed={level === f}
            onClick={() => setLevel(f)}
          >
            {f === "all" ? "all" : `${glyph[f]} ${f}`}
          </button>
        ))}
      </div>
      {groups.length === 0 && <p className="muted">No rules at this level.</p>}
      {groups.map((g) => (
        <div key={g.eco}>
          <h3 className="rule-eco">{g.eco}</h3>
          {g.rules.map((r) => {
            const match = matchText(r);
            return (
              <div className="rule" key={r.ID}>
                <span className={`lvl ${r.Safety}`}>
                  {glyph[r.Safety]} {r.Safety}
                </span>
                <div>
                  <span className="id">{r.ID}</span> <span className="name">· {r.Name}</span>
                </div>
                <p className="why">{r.Why}</p>
                {match && (
                  <p className="match">
                    {match}
                    {r.Restore && ` · restore: ${r.Restore}`}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      ))}
    </>
  );
}
