import { Link } from "react-router";

import type { TreeNode } from "../../services/page.service";

/**
 * SubpageList is pages as a list of links, in their order: a page's
 * children below its reading view, a notebook's root pages on its home
 * (M4/P5 design 3.5). It is the app's, not Markdown's: the tree read again
 * refreshes it.
 */
export function SubpageList({
  label,
  pages,
  href,
}: {
  label: string;
  pages: readonly TreeNode[];
  href: (id: string) => string;
}) {
  return (
    <ul aria-label={label} className="divide-y rounded-md border">
      {pages.map((page) => (
        <li key={page.id}>
          <Link to={href(page.id)} className="block truncate px-4 py-2 hover:bg-accent">
            {page.name}
          </Link>
        </li>
      ))}
    </ul>
  );
}
