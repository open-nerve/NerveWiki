import type { FieldMessage, FieldTexts } from "./problem-messages";

/** maxNotebookNameBytes is a notebook name's longest, in bytes of UTF-8: a file name's limit (shared.CheckTitle). */
const maxNotebookNameBytes = 255;

const utf8 = new TextEncoder();

/**
 * notebookNameProblem is why name cannot be sent: empty, or longer than 255
 * bytes once trimmed, in NFC, as the server counts them (M3/P1 design 3.2).
 * The characters a file name cannot hold, a dot at either end and a name
 * Windows reserves are the server's to find: its 422 shows under the field,
 * in notebookNameTexts.
 */
export function notebookNameProblem(name: string): FieldMessage | undefined {
  const title = name.trim().normalize("NFC");
  if (title === "") {
    return "field.required";
  }
  return utf8.encode(title).length > maxNotebookNameBytes ? "field.notebook_name.too_long" : undefined;
}

/**
 * notebookNameTexts say the server's problems with a notebook's name by the
 * rules of a title, which the field's global texts, those of the names a
 * person gives, do not (v0.1 design 13.2, item 11).
 */
export const notebookNameTexts = {
  "name.too_long": "field.notebook_name.too_long",
  "name.invalid_format": "field.notebook_name.invalid_format",
  "name.not_allowed": "field.notebook_name.not_allowed",
} as const satisfies FieldTexts;
