import { useCallback, type ReactNode } from "react";
import { DOCS_URL, HOME_URL, REPO, doc } from "../site";
import { GitHubIcon, MoonIcon } from "./Icons";

function useThemeToggle() {
  return useCallback(() => {
    const root = document.documentElement;
    root.dataset.theme = root.dataset.theme === "dark" ? "light" : "dark";
    try {
      localStorage.setItem("devsweep-theme", root.dataset.theme);
    } catch {
      /* storage unavailable: the toggle still works for this visit */
    }
  }, []);
}

type HeaderProps = { page: "home" | "docs" };

export function Header({ page }: HeaderProps) {
  const toggleTheme = useThemeToggle();
  return (
    <header className="site-header">
      <div className={page === "docs" ? "nav nav-wide" : "wrap nav"}>
        <a className="brand" href={HOME_URL}>
          devsweep
          {page === "docs" && <span className="crumb">/ Docs</span>}
        </a>
        <nav className="nav-links" aria-label="Main">
          {page === "home" && (
            <>
              <a href={DOCS_URL} className="hide-sm">Docs</a>
              <a href={doc("rules")} className="hide-sm">Rules</a>
            </>
          )}
          <a href={REPO} className="icon-btn" aria-label="devsweep on GitHub">
            <GitHubIcon />
          </a>
          <button className="icon-btn" type="button" onClick={toggleTheme} aria-label="Toggle dark mode">
            <MoonIcon />
          </button>
          {page === "home" && (
            <a href={doc("install")} className="btn btn-primary btn-sm">Get started</a>
          )}
        </nav>
      </div>
    </header>
  );
}

export function Footer({ wide = false, children }: { wide?: boolean; children?: ReactNode }) {
  return (
    <footer className="site-footer">
      <div className="wrap" style={wide ? { maxWidth: 1280 } : undefined}>
        <span className="mono">devsweep</span>
        <nav aria-label="Footer">
          {children}
          <a href={REPO}>GitHub</a>
          <a href={`${REPO}/issues`}>Issues</a>
        </nav>
      </div>
    </footer>
  );
}
