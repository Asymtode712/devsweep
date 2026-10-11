export const REPO = "https://github.com/Asymtode712/devsweep";
export const ISSUES = `${REPO}/issues`;
export const GO_INSTALL = "go install github.com/Asymtode712/devsweep@latest";

/** Paths relative to the page, so the build works under any base path. */
export const HOME_URL = "./";
export const DOCS_URL = "docs.html";
export const doc = (anchor: string) => `${DOCS_URL}#${anchor}`;
