import type { ReactNode } from "react";

import type { PlainKey, Translate } from "../i18n/i18n";
import { ApiError } from "../services/api";

/**
 * What each refusal that names the pages being edited says of them first:
 * a rename or a move would write their links again (M6/P4 design 6); an
 * upgrade cannot start while they are edited (M11/P1 design 4.4).
 */
const leads: Record<string, PlainKey> = {
  "linking.pages_locked": "page.pagesLocked",
  "wiki.upgrade_pages_locked": "upgrade.pagesLocked",
};

/**
 * pagesLocked is what linking.pages_locked or wiki.upgrade_pages_locked
 * says of the pages it names: why they stop the change, and who edits
 * each, the page by its title, the account itself when me is theirs;
 * undefined for any other error, and for a page title does not know,
 * which errorText says in general. An upgrade's start names 100 pages at
 * most; the server's detail counts them all, in its own words, which the
 * page does not show.
 */
export function pagesLocked(
  error: unknown,
  t: Translate,
  me: string | undefined,
  title: (pageId: string) => string | undefined
): ReactNode {
  const lead = error instanceof ApiError && error.code !== undefined ? leads[error.code] : undefined;
  const locks = lead === undefined ? undefined : (error as ApiError).problem?.locks;
  if (lead === undefined || locks === undefined || locks.length === 0) {
    return undefined;
  }
  const items: { id: string; text: string }[] = [];
  for (const lock of locks) {
    const page = title(lock.page_id);
    if (page === undefined) {
      return undefined;
    }
    items.push({
      id: lock.page_id,
      text:
        lock.user_id === me
          ? t("page.editingTitledSelf", { page })
          : t("page.lockedTitled", { name: lock.display_name, page }),
    });
  }
  return (
    <>
      <p>{t(lead)}</p>
      <ul className="mt-1 list-disc pl-5">
        {items.map((item) => (
          <li key={item.id}>{item.text}</li>
        ))}
      </ul>
    </>
  );
}
