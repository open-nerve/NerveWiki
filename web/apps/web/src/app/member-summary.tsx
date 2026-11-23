import { observer } from "mobx-react-lite";

import { formatDate } from "../i18n/format";
import { useT, type Translate } from "../i18n/i18n";
import { useStore } from "../stores/context";
import { cn } from "../lib/cn";
import { badgeClass, listTextClass } from "./settings-page";

/** A member as a list shows them: a workspace's or a notebook's. */
export type Member = { display_name: string; email: string | null; created_at: string };

/**
 * memberWho names member in the controls of their row and in the audit's
 * sentences: by the address too, where it shows, since two members may have
 * the same name.
 */
export function memberWho(member: Pick<Member, "display_name" | "email">, t: Translate): string {
  return member.email === null
    ? member.display_name
    : t("members.who", { name: member.display_name, email: member.email });
}

/**
 * MemberSummary is who a member is (M2/P6 design 3.3): the name, You for
 * the account's own row, the address unless the viewer is a guest, and when
 * they joined. The settings' alone: their lists' names are 14px, You their
 * badge, after the name's last word (M13/P3/S2, decision 850; 4.1.7). The
 * address breaks only where it is longer than the line, the date never: a
 * narrow row broke "Nov 24" inside the month (as a token's).
 */
export const MemberSummary = observer(function MemberSummary({ member, you }: { member: Member; you: boolean }) {
  const { preferences } = useStore();
  const t = useT();
  return (
    <div className={cn(listTextClass, "space-y-1")}>
      <p className="text-sm font-medium break-words">
        {member.display_name}
        {you && <span className={badgeClass}>{t("members.you")}</span>}
      </p>
      <p className="text-sm text-muted-foreground">
        {member.email !== null && (
          <>
            <span className="break-words">{member.email}</span> ·{" "}
          </>
        )}
        <span className="whitespace-nowrap">
          {t("members.joined", { date: formatDate(member.created_at, preferences.locale) })}
        </span>
      </p>
    </div>
  );
});
