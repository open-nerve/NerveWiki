import type { FieldMessage } from "./problem-messages";

/** maxSlugLength is the longest slug, as the server's domain.MaxSlugLength. */
const maxSlugLength = 48;

/** maxWorkspaceName is the most characters a workspace's name has, after its surrounding blanks are trimmed. */
const maxWorkspaceName = 80;

/**
 * slugProblem is why slug cannot be sent, as the server's ValidSlug has it:
 * 1–48 of a–z, 0–9, _ and -, with nothing folded. Whether it is reserved or
 * taken, only the server knows (checkWorkspaceSlug).
 */
export function slugProblem(slug: string): FieldMessage | undefined {
  if (slug === "") {
    return "field.required";
  }
  return slug.length <= maxSlugLength && /^[a-z0-9_-]+$/.test(slug) ? undefined : "field.slug.invalid_format";
}

/**
 * workspaceNameProblem is why name cannot be sent: empty, or longer than 80
 * characters once trimmed. Characters are counted as code points, as the
 * server counts runes. The server's too_long of a name speaks of the token
 * names' 100 characters (field.name.too_long): this check comes first.
 */
export function workspaceNameProblem(name: string): FieldMessage | undefined {
  const trimmed = name.trim();
  if (trimmed === "") {
    return "field.required";
  }
  return [...trimmed].length > maxWorkspaceName ? "field.workspace_name.too_long" : undefined;
}

/**
 * slugFrom is the slug a workspace's name suggests: in lower case, without
 * accents, each run of other characters a single -, at most 48 long. A name
 * of other scripts suggests none; the user types one.
 */
export function slugFrom(name: string): string {
  return name
    .normalize("NFKD")
    .replace(/\p{Mark}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, "-")
    .replace(/^-+/, "")
    .slice(0, maxSlugLength)
    .replace(/-+$/, "");
}
