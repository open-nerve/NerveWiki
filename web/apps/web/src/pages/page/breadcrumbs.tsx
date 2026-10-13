import { Link } from "react-router";

import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";

/**
 * Breadcrumbs is where the page is (M4/P5 design 3.5): its notebook, then
 * its ancestors from the root down, each a link, then the page itself,
 * marked as the current one; or, a part of the page shown (one of its
 * versions, M8 design 4.13), a link too, then that part, the current one.
 */
export function Breadcrumbs({
  notebook,
  ancestors,
  page,
  href,
  part,
}: {
  notebook: Notebook;
  ancestors: readonly TreeNode[];
  page: TreeNode;
  href: (id?: string) => string;
  /** The part of the page shown, named. */
  part?: string;
}) {
  const steps = [
    { key: notebook.id, to: href(), name: notebook.name },
    ...ancestors.map((ancestor) => ({ key: ancestor.id, to: href(ancestor.id), name: ancestor.name })),
    ...(part === undefined ? [] : [{ key: page.id, to: href(page.id), name: page.name }]),
  ];
  return <BreadcrumbTrail steps={steps} current={part ?? page.name} />;
}

/**
 * BreadcrumbTrail is a trail of steps, each a link, then where the reader
 * is (current), marked as the current one: a page's (Breadcrumbs), a trash
 * item's preview (its notebook, the trash, the item).
 */
export function BreadcrumbTrail({
  steps,
  current,
}: {
  steps: readonly { key: string; to: string; name: string }[];
  current: string;
}) {
  const t = useT();
  return (
    <nav aria-label={t("page.breadcrumb")}>
      <ol className="flex flex-wrap items-center gap-x-1 text-sm text-muted-foreground">
        {steps.map((step, i) => (
          <li key={step.key}>
            {i > 0 && <Separator />}
            <Link to={step.to} className="hover:underline">
              {step.name}
            </Link>
          </li>
        ))}
        <li className="text-foreground">
          {steps.length > 0 && <Separator />}
          <span aria-current="page">{current}</span>
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
