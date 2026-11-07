/**
 * An unresolved LLM target's virtual route (M10 design 4.9). This follows
 * the shared target grammar and strips one .md suffix. Only the four ASCII
 * reserved keys are needed here; the upload titleKey approximation would
 * incorrectly fold dotless ı into i. No URL decoding or whitespace trim.
 * An unknown ancestry cannot establish a relative target's root position.
 */
export function virtualTarget(raw: string, parentDepth: number | undefined): "index" | "log" | undefined {
  let rest = raw;
  let rooted = false;
  let relative = false;
  let up = 0;
  if (rest.startsWith("/")) {
    rooted = true;
    rest = rest.slice(1);
  } else if (rest.startsWith("./")) {
    relative = true;
    rest = rest.slice(2);
  }
  if (!rooted) {
    while (rest.startsWith("../")) {
      relative = true;
      up += 1;
      rest = rest.slice(3);
    }
  }
  const parts = rest.split("/");
  if (parts.some((part) => part === "" || part === "." || part === "..")) return undefined;
  if (relative && (parentDepth === undefined || up > parentDepth)) return undefined;
  const depth = relative ? (parentDepth ?? 0) - up : 0;
  if (depth + parts.length !== 1) return undefined;
  const last = parts[0];
  if (last === undefined) return undefined;
  let key = last.normalize("NFC").replace(/[A-Z]/g, (letter) => letter.toLowerCase());
  if (key.length > 3 && key.endsWith(".md")) key = key.slice(0, -3);
  if (key === "index" || key === "index.md") return "index";
  if (key === "log" || key === "log.md") return "log";
  return undefined;
}
