import { useEffect, useState } from "react";
import { CheckIcon, CopyIcon } from "./Icons";

type Props = { text: string; label?: string; className?: string };

export function CopyButton({ text, label = "Copy", className = "icon-btn" }: Props) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const t = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(t);
  }, [copied]);

  const copy = () => {
    navigator.clipboard?.writeText(text).then(() => setCopied(true), () => {});
  };

  return (
    <button className={className} type="button" onClick={copy} aria-label={copied ? "Copied" : label}>
      {copied ? <CheckIcon /> : <CopyIcon />}
    </button>
  );
}
