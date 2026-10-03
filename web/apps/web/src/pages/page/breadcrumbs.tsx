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
          <li key={ancestor.id}>
            <Separator />
            <Link to={href(ancestor.id)} className="hover:underline">
              {ancestor.name}
            </Link>
          </li>
        ))}
        <li className="text-foreground">
          <Separator />
          <span aria-current="page">{page.name}</span>
        </li>
      </ol>
    </nav>
  );
}

/** Separator stands between two steps, for the eye: screen readers have the list. */
function Separator() {
  return (
    <span aria-hidden className="pr-1 text-muted-foreground">
      /
    </span>
  );
}
