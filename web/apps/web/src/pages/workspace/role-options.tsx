import { useT } from "../../i18n/i18n";
import type { WorkspaceRole } from "../../services/member.service";

/** A workspace's roles, from the most rights to the fewest. */
export const roles: readonly WorkspaceRole[] = ["admin", "member", "guest"];

/** RoleOptions are the options of a select of a workspace's roles. */
export function RoleOptions() {
  const t = useT();
  return roles.map((role) => (
    <option key={role} value={role}>
      {t(`role.${role}`)}
    </option>
  ));
}
