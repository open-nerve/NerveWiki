import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";

/**
 * Breadcrumbs is where the page is (M4/P5 design 3.5): its notebook, then
 * its ancestors from the root down, each a link, then the page itself,
 * marked as the current one.
 */
export function Breadcrumbs({
  notebook,
  ancestors,
  page,
  href,
}: {
  notebook: Notebook;
  ancestors: readonly TreeNode[];
  page: TreeNode;
  href: (id?: string) => string;
}) {
  const t = useT();
  return (
    <nav aria-label={t("page.breadcrumb")}>
      <ol className="flex flex-wrap items-center gap-x-1 text-sm text-muted-foreground">
        <li>
          <Link to={href()} className="hover:underline">
            {notebook.name}
          </Link>
        </li>
        {ancestors.map((ancestor) => (
          <li key={ancestor.id} className="before:pr-1 before:content-['/']">
            <Link to={href(ancestor.id)} className="hover:underline">
              {ancestor.name}
            </Link>
          </li>
        ))}
        <li
          aria-current="page"
          className="text-foreground before:pr-1 before:text-muted-foreground before:content-['/']"
        >
          {page.name}
        </li>
      </ol>
    </nav>
  );
}
