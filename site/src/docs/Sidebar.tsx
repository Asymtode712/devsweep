import { useEffect, useRef, useState } from "react";
import { SearchIcon } from "../components/Icons";

export type NavGroup = { title: string; links: { id: string; label: string }[] };

/** Tracks which section is in view, for highlighting the sidebar. */
function useActiveSection(ids: string[]) {
  const [active, setActive] = useState(ids[0]);
  useEffect(() => {
    if (!("IntersectionObserver" in window)) return;
    const visible = new Map<string, boolean>();
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) visible.set(e.target.id, e.isIntersecting);
        const first = ids.find((id) => visible.get(id));
        if (first) setActive(first);
      },
      { rootMargin: "-80px 0px -55% 0px" },
    );
    for (const id of ids) {
      const el = document.getElementById(id);
      if (el) io.observe(el);
    }
    return () => io.disconnect();
  }, [ids]);
  return active;
}

/** Matches the query against each link's label and its section's text. */
function useSectionSearch(query: string, ids: string[]) {
  const [hits, setHits] = useState<Set<string> | null>(null);
  useEffect(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      setHits(null);
      return;
    }
    setHits(new Set(ids.filter((id) => document.getElementById(id)?.textContent?.toLowerCase().includes(q))));
  }, [query, ids]);
  return hits;
}

export function Sidebar({ groups }: { groups: NavGroup[] }) {
  const ids = useRef(groups.flatMap((g) => g.links.map((l) => l.id))).current;
  const active = useActiveSection(ids);
  const [query, setQuery] = useState("");
  const hits = useSectionSearch(query, ids);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (document.activeElement?.tagName ?? "").toLowerCase();
      if (e.key === "/" && tag !== "input" && tag !== "textarea") {
        e.preventDefault();
        input.current?.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const shown = (id: string, label: string) =>
    !hits || hits.has(id) || label.toLowerCase().includes(query.trim().toLowerCase());
  const anyShown = groups.some((g) => g.links.some((l) => shown(l.id, l.label)));

  return (
    <aside className="sidebar" aria-label="Documentation">
      <div className="sidebar-inner">
        <label className="search">
          <SearchIcon />
          <input
            ref={input}
            type="search"
            placeholder="Search docs"
            aria-label="Search docs"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <kbd aria-hidden="true">/</kbd>
        </label>
        <div className="nav-groups">
          {groups.map((g) => {
            const links = g.links.filter((l) => shown(l.id, l.label));
            if (!links.length) return null;
            return (
              <nav className="nav-group" aria-label={g.title} key={g.title}>
                <span className="title">{g.title.toUpperCase()}</span>
                {links.map((l) => (
                  <a
                    key={l.id}
                    href={`#${l.id}`}
                    className={active === l.id ? "active" : undefined}
                    aria-current={active === l.id ? "true" : undefined}
                  >
                    {l.label}
                  </a>
                ))}
              </nav>
            );
          })}
        </div>
        {!anyShown && <p className="nav-empty">No matches.</p>}
      </div>
    </aside>
  );
}
