import type { Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import type { SearchCount, SearchPage } from "../../services/page.service";

/** The count's largest value, the server's: 1,000 is always at least, said "1,000+". */
const countCap = 1000;

/**
 * statusText is what a search's status line says of the pages of it read
 * (M8/P5 design 4.3), listing found hits, each once. Of the first page
 * alone, its count: exact (none found, or how many), about, at least
 * (1,000 as "1,000+"), or none found yet while there is more to look
 * through. Of more pages, by what was read: read to the end, how many were
 * found; a count that says none or fewer than were found, how many so far;
 * else the first page's count. While the index is being built, it says
 * that the hits may be missing some.
 */
export function statusText(pages: readonly SearchPage[], found: number, t: Translate, locale: Locale): string {
  const number = (n: number) => new Intl.NumberFormat(locale).format(n);
  const count = pages[0]?.count ?? null;
  const more = pages.length > 1;
  let text: string;
  if (more && (pages.at(-1)?.next_cursor ?? null) === null) {
    text = found === 0 ? t("search.none") : t("search.count", { count: number(found) });
  } else if (more && found > 0 && (count === null || count.value < found)) {
    text = t("search.soFar", { count: number(found) });
  } else {
    text = countText(count, t, number);
  }
  return pages.some((page) => page.building) ? t("search.building", { status: text }) : text;
}

/** countText says a first page's count, its numbers written by number. */
function countText(count: SearchCount | null, t: Translate, number: (n: number) => string): string {
  if (count === null) {
    return t("search.noneYet");
  }
  switch (count.kind) {
    case "exact":
      return count.value === 0 ? t("search.none") : t("search.count", { count: number(count.value) });
    case "about":
      return t("search.about", { count: number(count.value) });
    case "at_least":
      return count.value >= countCap
        ? t("search.capped", { count: number(count.value) })
        : t("search.atLeast", { count: number(count.value) });
  }
}

/** moreText is what is said once More or Keep looking read another page: how many hits it added, or that it found none. */
export function moreText(added: number, t: Translate, locale: Locale): string {
  return added === 0
    ? t("search.noneHere")
    : t("search.moreFound", { count: new Intl.NumberFormat(locale).format(added) });
}
