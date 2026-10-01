import { useT } from "../../i18n/i18n";
import type { NotebookRole } from "../../services/notebook.service";

/** A notebook's roles, from the most rights to the fewest. */
export const notebookRoles: readonly NotebookRole[] = ["admin", "editor", "reader"];

/** NotebookRoleOptions are the options of a select of a notebook's roles. */
export function NotebookRoleOptions() {
  const t = useT();
  return notebookRoles.map((role) => (
    <option key={role} value={role}>
      {t(`notebookRole.${role}`)}
    </option>
  ));
}
