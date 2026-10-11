import type { CSSProperties } from "react";
import { CopyButton } from "./CopyButton";

/**
 * A shell snippet. Lines starting with "$ " get a dimmed prompt that isn't copied;
 * lines starting with "#" are comments; anything else is output.
 */
type Props = { code: string; copy?: boolean; style?: CSSProperties };

export function CodeBlock({ code, copy = false, style }: Props) {
  const lines = code.replace(/^\n|\n$/g, "").split("\n");
  const commands = lines.filter((l) => l.startsWith("$ ")).map((l) => l.slice(2));

  return (
    <pre className="codeblock" style={style}>
      {copy && commands.length > 0 && (
        <CopyButton text={commands.join("\n")} label="Copy commands" className="icon-btn copy" />
      )}
      {lines.map((line, i) => (
        <span key={i}>
          {line.startsWith("$ ") ? (
            <>
              <span className="p">$ </span>
              {line.slice(2)}
            </>
          ) : line.startsWith("#") ? (
            <span className="c">{line}</span>
          ) : (
            line
          )}
          {i < lines.length - 1 && "\n"}
        </span>
      ))}
    </pre>
  );
}
