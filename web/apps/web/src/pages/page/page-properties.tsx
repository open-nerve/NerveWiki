import type { ReactNode } from "react";
import { Link } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import type { PageProperties as Properties } from "../../services/linking.service";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { PanelSection } from "./panel-section";

/** A value as the properties show it: text, or a property link's text and the page it leads to (null: none). */
type Shown = string | { text: string; lead: string | null };

/** Take answers where the next property link written at path leads, undefined for no more there. */
type Take = (path: string) => string | null | undefined;

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
    shown = (
      <div className="text-sm">
        <NotLoaded error={error} retry={() => void mutate()} />
      </div>
    );
  } else if (!data.valid) {
    shown = <p className="text-sm text-muted-foreground">{t("page.propertiesInvalid")}</p>;
  } else if (data.properties.length === 0) {
    shown = <p className="text-sm text-muted-foreground">{t("page.noProperties")}</p>;
  } else {
    const take = taking(data.links);
    shown = (
      <dl className="space-y-2 text-sm">
        {data.properties.map(({ key, value }) => (
          <div key={key}>
            <dt className="break-words text-muted-foreground">{key}</dt>
            <dd className="break-words">
              {Array.isArray(value) ? (
                <ul>
                  {value.map((item: unknown, at) => (
                    // oxlint-disable-next-line react/no-array-index-key -- a list's items may repeat: by where they are
                    <li key={at}>{show(shownOf(item, `${key}.${at}`, take), href)}</li>
                  ))}
                </ul>
              ) : (
                show(shownOf(value, key, take), href)
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
 * taking takes the property links by their paths, each path's in the order
 * written: a path two values share (a key "a.0", a list a's first item) is
 * theirs in turn, as the values are read in the order written (M6 design
 * 4.9 aligns the table's by the values for this).
 */
function taking(links: Properties["links"]): Take {
  const byPath = new Map<string, (string | null)[]>();
  for (const { key, node_id: node } of links) {
    const queue = byPath.get(key);
    if (queue === undefined) {
      byPath.set(key, [node ?? null]);
    } else {
      queue.push(node ?? null);
    }
  }
  return (path) => byPath.get(path)?.shift();
}

/**
 * shownOf is the value at path as shown, the property links it holds taken
 * in the order written: a string as it is, unless it is a property link; a
 * number or a boolean as JSON writes it; null as nothing; anything else,
 * an object or a list in a list, as its JSON, its links taken all the same.
 */
function shownOf(value: unknown, path: string, take: Take): Shown {
  if (typeof value === "string") {
    const lead = linkLike(value) ? take(path) : undefined;
    return lead === undefined ? value : { text: linkText(value), lead };
  }
  passOver(value, path, take);
  return value === null ? "" : JSON.stringify(value);
}

/** passOver takes the property links value holds, at path and under it, as an object or a list shows none. */
function passOver(value: unknown, path: string, take: Take) {
  if (typeof value === "string") {
    if (linkLike(value)) {
      take(path);
    }
  } else if (Array.isArray(value)) {
    value.forEach((item: unknown, at) => passOver(item, `${path}.${at}`, take));
  } else if (typeof value === "object" && value !== null) {
    for (const [key, item] of Object.entries(value)) {
      passOver(item, `${path}.${key}`, take);
    }
  }
}

/** linkLike tells whether a string may be a property link: one starting with '[', with no space around it (server's rule). */
function linkLike(value: string): boolean {
  return value.startsWith("[") && value.trim() === value;
}

/** show is a value shown: its text, or its link, leading to its page or styled as one to none. */
function show(shown: Shown, href: (id: string) => string): ReactNode {
  if (typeof shown === "string") {
    return shown;
  }
  return shown.lead === null ? (
    <span className="text-muted-foreground underline decoration-dashed underline-offset-4">{shown.text}</span>
  ) : (
    <Link to={href(shown.lead)} state={arrived} className="underline underline-offset-4">
      {shown.text}
    </Link>
  );
}

/**
 * linkText is the text a property link shows, from the link it is written
 * as: a wikilink's display text, or its target, then its anchor after
 * " > ", each without the spaces and tabs around it (a table's \| ends the
 * target too), as the reading view writes it (M6/P6 design 4); a Markdown
 * link's text as written, its escapes, references and marks kept, where
 * the reading view's property table shows the text they make (accepted:
 * the index keeps no link's text).
 */
function linkText(written: string): string {
  if (!(written.startsWith("[[") && written.endsWith("]]"))) {
    return /^\[(.*?)\]\(/s.exec(written)?.[1] ?? written;
  }
  const inner = written.slice(2, -2);
  const bar = inner.indexOf("|");
  if (bar !== -1 && trimmed(inner.slice(bar + 1)) !== "") {
    return trimmed(inner.slice(bar + 1));
  }
  const link = bar === -1 ? inner : inner.slice(0, bar > 0 && inner[bar - 1] === "\\" ? bar - 1 : bar);
  const hash = link.indexOf("#");
  // A property link has a target: one to an anchor of its own page alone is none.
  const [target, anchor] = hash === -1 ? [link, ""] : [link.slice(0, hash), trimmed(link.slice(hash + 1))];
  return anchor === "" ? trimmed(target) : `${trimmed(target)} > ${anchor}`;
}

/** trimmed is text without the spaces and tabs at its ends, as a wikilink's parts are read. */
function trimmed(text: string): string {
  return text.replace(/^[ \t]+|[ \t]+$/g, "");
}
