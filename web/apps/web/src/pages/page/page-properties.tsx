import type { ReactNode } from "react";
import { Link } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { PanelSection } from "./panel-section";

/** Where each property link leads, by its path: a page, or null for none. */
type Leads = ReadonlyMap<string, string | null>;

/**
 * PageProperties is the page's frontmatter properties as the link index has
 * them (M6/P7 design 10), in the order written: each its key and its
 * value. A property link, a value or a list's item that is one link, shows
 * the link's text, leading to the page it resolves to, or, resolving to
 * none, styled as a link to no page is. A frontmatter that is not valid
 * says so, as does one without properties.
 */
export function PageProperties({
  notebook,
  page,
  href,
}: {
  notebook: Notebook;
  page: string;
  href: (id: string) => string;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(["page-properties", notebook.id, page], () => pages.properties(page));
  let shown: ReactNode;
  if (data === undefined) {
    shown = <NotLoaded error={error} retry={() => void mutate()} />;
  } else if (!data.valid) {
    shown = <p className="text-sm text-muted-foreground">{t("page.propertiesInvalid")}</p>;
  } else if (data.properties.length === 0) {
    shown = <p className="text-sm text-muted-foreground">{t("page.noProperties")}</p>;
  } else {
    const leads = new Map<string, string | null>();
    for (const { key, node_id: node } of data.links) {
      if (!leads.has(key)) {
        leads.set(key, node ?? null);
      }
    }
    shown = (
      <dl className="space-y-2 text-sm">
        {data.properties.map(({ key, value }, index) => (
          // oxlint-disable-next-line react/no-array-index-key -- the list is read whole, in the order written
          <div key={index}>
            <dt className="break-words text-muted-foreground">{key}</dt>
            <dd className="break-words">
              {Array.isArray(value) ? (
                <ul>
                  {value.map((item: unknown, at) => (
                    // oxlint-disable-next-line react/no-array-index-key -- a list's items may repeat: by where they are
                    <li key={at}>{scalar(item, `${key}.${at}`, leads, href)}</li>
                  ))}
                </ul>
              ) : (
                scalar(value, key, leads, href)
              )}
            </dd>
          </div>
        ))}
      </dl>
    );
  }
  return <PanelSection title={t("page.properties")}>{shown}</PanelSection>;
}

/**
 * scalar shows the value at path: a string as it is, unless it is a
 * property link; a number or a boolean as JSON writes it; null as
 * nothing; anything else, an object or a list in a list, as its JSON.
 */
function scalar(value: unknown, path: string, leads: Leads, href: (id: string) => string): ReactNode {
  if (typeof value === "string") {
    const lead = leads.get(path);
    if (lead === undefined) {
      return value;
    }
    return lead === null ? (
      <span className="text-muted-foreground underline decoration-dashed underline-offset-4">{linkText(value)}</span>
    ) : (
      <Link to={href(lead)} state={arrived} className="underline underline-offset-4">
        {linkText(value)}
      </Link>
    );
  }
  return value === null ? null : JSON.stringify(value);
}

/**
 * linkText is the text a property link shows (M6/P6 design 4), from the
 * link it is written as: a wikilink's display, or its target, then its
 * anchor after " > ", as the reading view writes it; a Markdown link's
 * text as written.
 */
function linkText(written: string): string {
  if (written.startsWith("[[") && written.endsWith("]]")) {
    const inner = written.slice(2, -2);
    const bar = inner.indexOf("|");
    const display = bar === -1 ? "" : inner.slice(bar + 1);
    if (display !== "") {
      return display;
    }
    const link = bar === -1 ? inner : inner.slice(0, bar);
    const hash = link.indexOf("#");
    if (hash === -1) {
      return link;
    }
    const [target, anchor] = [link.slice(0, hash), link.slice(hash + 1)];
    return target === "" ? anchor : `${target} > ${anchor}`;
  }
  return /^\[(.*)\]\(/s.exec(written)?.[1] ?? written;
}
