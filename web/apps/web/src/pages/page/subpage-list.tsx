import { memo, type MouseEvent } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { useShownRows } from "../../app/shown-rows";
import { linkFollowed } from "../../lib/link-click";
import type { TreeNode } from "../../services/page.service";

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
 */
export function SubpageList({
  label,
  pages,
  href,
  nameOf = title,
}: {
  label: string;
  pages: readonly TreeNode[];
  href: (id: string) => string;
  nameOf?: (page: TreeNode) => string;
}) {
  const navigate = useNavigate();
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
    <ul aria-label={label} className="divide-y rounded-md border" onClick={follow}>
      {pages.slice(0, shown).map((page) => (
        <SubpageRow key={page.id} href={href(page.id)} name={nameOf(page)} />
      ))}
    </ul>
  );
}

/** SubpageRow is a page's row of the list: a link to it, by name. */
const SubpageRow = memo(function SubpageRow({ href, name }: { href: string; name: string }) {
  return (
    <li>
      <a href={href} className="block truncate px-4 py-2 hover:bg-hover">
        {name}
      </a>
    </li>
  );
});

/** title is a page's title, how the list names a page by default. */
function title(page: TreeNode): string {
  return page.name;
}
