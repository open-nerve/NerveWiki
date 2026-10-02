import { observer } from "mobx-react-lite";

import { formatDate } from "../i18n/format";
import { useT, type Translate } from "../i18n/i18n";
import { useStore } from "../stores/context";

/** A member as a list shows them: a workspace's or a notebook's. */
type Member = { display_name: string; email: string | null; created_at: string };

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
 * they joined.
 */
export const MemberSummary = observer(function MemberSummary({ member, you }: { member: Member; you: boolean }) {
  const { preferences } = useStore();
  const t = useT();
  return (
    <div className="min-w-0 space-y-1">
      <p className="flex flex-wrap items-center gap-2 font-medium break-words">
        {member.display_name}
        {you && (
          <span className="rounded border px-1.5 text-xs font-normal text-muted-foreground">{t("members.you")}</span>
        )}
      </p>
      <p className="text-sm break-all text-muted-foreground">
        {member.email !== null && <>{member.email} · </>}
        {t("members.joined", { date: formatDate(member.created_at, preferences.locale) })}
      </p>
    </div>
  );
});
