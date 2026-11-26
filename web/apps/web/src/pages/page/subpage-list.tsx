import { memo, useId, type MouseEvent } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { cardClass } from "../../app/settings-page";
import { useShownRows } from "../../app/shown-rows";
import { PageIcon } from "../../components/page-icon";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import { linkFollowed } from "../../lib/link-click";
import type { TreeNode } from "../../services/page.service";
import { useStore } from "../../stores/context";

/**
 * SubpageList is pages as a list of links, in their order: a page's
 * children below its reading view, a notebook's root pages on its home
 * (M4/P5 design 3.5), a tag's pages (M6/P6 design 8), each by its title or
 * the name nameOf gives. It is the app's, not Markdown's: the tree read
 * again refreshes it. A link leaves with the page it is on: the page it
 * leads to is arrived at, its title taking the focus (v0.1 design 13.2,
 * item 17; M5-polish 12.3).
 *
 * Ten thousand of them are cheap (M12/P4/S4 design 3.7): each row a memo
 * of its address and name, its link a plain anchor the list follows as a
 * router's link would; the list draws its first rows at once and the rest
 * a chunk after each paint (useShownRows).
 *
 * A card, a row a page, its icon before its name, cut short on one line
 * (M13/P5/S1, decisions 919, 931). Given countOf (the notebook's home), a
 * row of a page with pages under it says how many at its end, outside its
 * link and read by no one, the link described by a hidden sentence that
 * says it: each row's strings made here, once a render, the rows comparing
 * strings alone.
 */
export function SubpageList({
  label,
  pages,
  href,
  nameOf = title,
  countOf,
}: {
  label: string;
  pages: readonly TreeNode[];
  href: (id: string) => string;
  nameOf?: (page: TreeNode) => string;
  countOf?: (page: TreeNode) => number;
}) {
  const navigate = useNavigate();
  const t = useT();
  const { locale } = useStore().preferences;
  const shown = useShownRows(pages.length);
  const follow = (event: MouseEvent<HTMLUListElement>) => {
    const to = linkFollowed(event);
    if (to !== undefined) {
      event.preventDefault();
      void navigate(to, { state: arrived });
    }
  };
  return (
    // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions -- a click on one of its links, which Enter on a link is too
    <ul aria-label={label} className={cardClass} onClick={follow}>
      {pages.slice(0, shown).map((page) => {
        const n = countOf?.(page) ?? 0;
        return (
          <SubpageRow
            key={page.id}
            href={href(page.id)}
            name={nameOf(page)}
            {...(n === 0
              ? {}
              : {
                  count: n.toLocaleString(locale),
                  said:
                    n === 1 ? t("notebook.subpagesOne") : t("notebook.subpages", { count: n.toLocaleString(locale) }),
                })}
          />
        );
      })}
    </ul>
  );
}

/**
 * SubpageRow is a page's row of the list: a link to it, by name, its icon before it; count, the number seen at the
 * row's end, read by no one, said, the sentence that describes the link. The row's hover fills its card's corners, 1px
 * inside its 14px; the link's focus is drawn inside the row, in the card (v0.1 design 13.2, item 31; review B's
 * P5B-M3).
 */
const SubpageRow = memo(function SubpageRow({
  href,
  name,
  count,
  said,
}: {
  href: string;
  name: string;
  count?: string;
  said?: string;
}) {
  const described = useId();
  return (
    <li className="relative transition-colors duration-150 first:rounded-t-[13px] last:rounded-b-[13px] hover:bg-hover">
      <a
        href={href}
        aria-describedby={said === undefined ? undefined : described}
        className={cn(
          "flex items-center gap-2.5 rounded-[inherit] py-2.5 ps-4.5 text-sm focus-visible:-outline-offset-2",
          count === undefined ? "pe-4.5" : "pe-16"
        )}
      >
        <PageIcon className="text-faint" />
        <span title={name} className="min-w-0 truncate">
          {name}
        </span>
      </a>
      {count !== undefined && (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-0 end-4.5 flex items-center text-compact text-faint tabular-nums"
        >
          {count}
        </span>
      )}
      {said !== undefined && (
        <span id={described} hidden>
          {said}
        </span>
      )}
    </li>
  );
});

/** title is a page's title, how the list names a page by default. */
function title(page: TreeNode): string {
  return page.name;
}
