import type { ReactNode } from "react";

import type { Translate } from "../i18n/i18n";
import { ApiError } from "../services/api";

/**
 * pagesLocked is what linking.pages_locked says of the pages it names (M6/P4
 * design 6): a rename or move writes their links again, and who edits each,
 * the page by its title, the account itself when me is theirs; undefined
 * for any other error, and for a page title does not know, which errorText
 * says in general.
 */
export function pagesLocked(
  error: unknown,
  t: Translate,
  me: string | undefined,
  title: (pageId: string) => string | undefined
): ReactNode {
  const locks = error instanceof ApiError && error.code === "linking.pages_locked" ? error.problem?.locks : undefined;
  if (locks === undefined || locks.length === 0) {
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
      <p>{t("page.pagesLocked")}</p>
      <ul className="mt-1 list-disc pl-5">
        {items.map((item) => (
          <li key={item.id}>{item.text}</li>
        ))}
      </ul>
    </>
  );
}
