import { Link } from "react-router";

import { arrived } from "../../app/arrival";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";

/**
 * Breadcrumbs is where the page is (M4/P5 design 3.5): its notebook, then
 * its ancestors from the root down, each a link, then the page itself,
 * marked as the current one; or, a part of the page shown (one of its
 * versions, M8 design 4.13), a link too, then that part, the current one.
 * className places its trail in a row (the page's head, M13/P4/S1).
 */
export function Breadcrumbs({
  notebook,
  ancestors,
  page,
  href,
  part,
  className,
}: {
  notebook: Notebook;
  ancestors: readonly TreeNode[];
  page: TreeNode;
  href: (id?: string) => string;
  /** The part of the page shown, named. */
  part?: string;
  className?: string;
}) {
  const steps = [
    { key: notebook.id, to: href(), name: notebook.name },
    ...ancestors.map((ancestor) => ({ key: ancestor.id, to: href(ancestor.id), name: ancestor.name })),
    ...(part === undefined ? [] : [{ key: page.id, to: href(page.id), name: page.name }]),
  ];
  return <BreadcrumbTrail steps={steps} current={part ?? page.name} className={className} />;
}

/**
 * BreadcrumbTrail is a trail of steps, each a link, then where the reader
 * is (current), marked as the current one: a page's (Breadcrumbs), a trash
 * item's preview (its notebook, the trash, the item). A step leaves with
 * the page it is on: the page it leads to is arrived at, its title taking
 * the focus (v0.1 design 13.2, item 17; M5-polish 12.3). Faint and 13px, as the mockup's .crumb: a step lights up
 * as it is hovered, unlined (00-M13 4.7), where the reader is in the secondary text and heavier (M13/P4/S1).
 * className is its nav's. Each step's separator ends its link (Separator).
 */
export function BreadcrumbTrail({
  steps,
  current,
  className,
}: {
  steps: readonly { key: string; to: string; name: string }[];
  current: string;
  className?: string;
}) {
  const t = useT();
  return (
    <nav aria-label={t("page.breadcrumb")} className={className}>
      <ol className="flex flex-wrap items-center gap-x-1 text-compact text-faint [overflow-wrap:anywhere]">
        {steps.map((step) => (
          <li key={step.key}>
            <Link to={step.to} state={arrived} className="hover:text-foreground">
              {step.name}
              <Separator />
            </Link>
          </li>
        ))}
        <li className="font-medium text-muted-foreground">
          <span aria-current="page">{current}</span>
        </li>
      </ol>
    </nav>
  );
}

/**
 * Separator stands between two steps, for the eye: screen readers have the list, and a link's name is its step's
 * alone. It ends the step before it, in its link: the line breaks after it, between two steps, a long name broken in
 * its own; before its step, a sibling of it, Firefox broke the line after it, leaving it alone on a line at any width
 * (M13 closeout, review B's M13CB-M2, decision 968).
 */
function Separator() {
  return (
    <span aria-hidden className="ps-1 font-normal text-faint">
      /
    </span>
  );
}
