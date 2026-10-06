import { Link } from "react-router";

import type { TreeNode } from "../../services/page.service";

/**
 * SubpageList is pages as a list of links, in their order: a page's
 * children below its reading view, a notebook's root pages on its home
 * (M4/P5 design 3.5), a tag's pages (M6/P6 design 8), each by its title or
 * the name nameOf gives. It is the app's, not Markdown's: the tree read
 * again refreshes it.
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
  return (
    <ul aria-label={label} className="divide-y rounded-md border">
      {pages.map((page) => (
        <li key={page.id}>
          <Link to={href(page.id)} className="block truncate px-4 py-2 hover:bg-accent">
            {nameOf(page)}
          </Link>
        </li>
      ))}
    </ul>
  );
}

/** title is a page's title, how the list names a page by default. */
function title(page: TreeNode): string {
  return page.name;
}
