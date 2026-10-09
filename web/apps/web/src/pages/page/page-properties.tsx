import { useMemo, type ReactNode } from "react";
import { Link } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import type { PageProperties as Properties } from "../../services/linking.service";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { eachRead, stamped, useAssetsExpiry } from "./assets-expiry";
import { PanelSection } from "./panel-section";

/** A value as the properties show it: text, or a property link's text and what it leads to. */
type Shown = string | { text: string; lead: Lead };

/**
 * What a property link leads to: a page, by its id; none (null); or an
 * attachment, at its content's address, which the server signs, and
 * whether the browser shows it there (M7/P4 design 4.3); null for none:
 * one deleted since its link was indexed (M7/P3 design 5.6).
 */
type Lead = string | null | { asset: null } | { asset: string; inline: boolean };

/**
 * How long a property link's path may be, in UTF-16 code units, to be
 * paired with its value: one longer shows as its text (the reading view's
 * property table has it as a link; a list's items may be either, by how
 * long their paths are). A browser's map costs the square of their number
 * for many strings that long, which a writer could have every reader's tab
 * wait on.
 */
const pathsUpTo = 1024;

/** A property as shown: its key, and its value, or its list's items, as shown. */
type Row = { key: string; shown: Shown | Shown[] };

/** Take answers where the property link a string at path is leads, undefined when it is none. */
type Take = (value: string, path: string) => Lead | undefined;

/**
 * PageProperties is the page's frontmatter properties as the link index has
 * them (M6/P7 design 10), in the order written: each its key and its
 * value. A property link, a value or a list's item that is one link, shows
 * the link's text, leading to the page it resolves to, or, resolving to
 * none, styled as a link to no page is; one to an attachment leads to its
 * content (M7/P3 design 5.6). A frontmatter that is not valid says so, as
 * does one without properties. Their attachments' addresses expiring, the
 * properties are read again before they do; the cache's, expired, are none
 * until they are (M7/P4 design 4.3).
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
  const answer = useSWR(["page-properties", notebook.id, page], () => stamped(pages.properties(page)), eachRead);
  const { error, mutate } = answer;
  const data = useAssetsExpiry(answer.data, () => void mutate());
  // Once for each answer, not at each render: the edit entered or left, the tree read again render the column.
  const rows = useMemo(() => (data?.valid === true ? rowsOf(data) : []), [data]);
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
    shown = (
      <dl aria-label={t("page.properties")} className="space-y-2 text-sm">
        {rows.map(({ key, shown: value }) => (
          <div key={key}>
            <dt className="break-words text-muted-foreground">{key}</dt>
            <dd className="break-words">
              {Array.isArray(value) ? (
                <ul>
                  {value.map((item, at) => (
                    // oxlint-disable-next-line react/no-array-index-key -- a list's items may repeat: by where they are
                    <li key={at}>{show(item, href, t("asset.newTab"))}</li>
                  ))}
                </ul>
              ) : (
                show(value, href, t("asset.newTab"))
              )}
            </dd>
          </div>
        ))}
      </dl>
    );
  }
  return <PanelSection title={t("page.properties")}>{shown}</PanelSection>;
}

/** rowsOf is each property as shown, its property links taken (taking). */
function rowsOf(data: Properties): Row[] {
  const take = taking(data);
  return data.properties.map(({ key, value }) => ({
    key,
    shown: Array.isArray(value)
      ? value.map((item: unknown, at) => shownOf(item, `${key}.${at}`, take))
      : shownOf(value, key, take),
  }));
}

/**
 * taking takes the property links by their paths, each path's in the order
 * written. A string at a path no other string is at has its path's link,
 * as the server pairs them. A path two share (a key "a.0", a list a's
 * first item) has its links theirs in turn, as the values are read in the
 * order written (an object's only hold their places: M6 design 4.9 aligns
 * the table's by the values for this): there one takes a link if it has a
 * link's shape. A path longer than pathsUpTo is not looked up.
 */
function taking({ properties, links }: Properties): Take {
  const byPath = new Map<string, Lead[]>();
  for (const { key, node_id: node, kind, url, inline } of links) {
    if (key.length > pathsUpTo) {
      continue;
    }
    let lead: Lead = node ?? null;
    if (kind === "asset") {
      lead = url === null ? { asset: null } : { asset: url, inline: inline === true };
    }
    const queue = byPath.get(key);
    if (queue === undefined) {
      byPath.set(key, [lead]);
    } else {
      queue.push(lead);
    }
  }
  const shared = sharedPaths(properties, byPath);
  return (value, path) => {
    const queue = path.length > pathsUpTo ? undefined : byPath.get(path);
    return queue === undefined || (shared.has(path) && !linkLike(value)) ? undefined : queue.shift();
  };
}

/**
 * sharedPaths are the paths of links two strings or more of properties are
 * at, as shownOf goes through them. Only a link's path is kept, up to
 * pathsUpTo long: what a set of the paths of all would cost is the square
 * of their number where many are long (V8 hashes a string that long by its
 * length).
 */
function sharedPaths(properties: Properties["properties"], linked: ReadonlyMap<string, unknown>): Set<string> {
  const seen = new Set<string>();
  const shared = new Set<string>();
  const visit = (value: unknown, path: string) => {
    if (typeof value === "string") {
      if (path.length <= pathsUpTo && linked.has(path)) {
        (seen.has(path) ? shared : seen).add(path);
      }
    } else if (Array.isArray(value)) {
      value.forEach((item: unknown, at) => visit(item, `${path}.${at}`));
    } else if (typeof value === "object" && value !== null) {
      for (const [key, item] of Object.entries(value)) {
        visit(item, `${path}.${key}`);
      }
    }
  };
  for (const { key, value } of properties) {
    visit(value, key);
  }
  return shared;
}

/**
 * shownOf is the value at path as shown, the property links it holds taken
 * in the order written: a string as it is, unless it is a property link; a
 * number or a boolean as JSON writes it; null as nothing; anything else,
 * an object or a list in a list, as its JSON, its links taken all the same.
 */
function shownOf(value: unknown, path: string, take: Take): Shown {
  if (typeof value === "string") {
    const lead = take(value, path);
    return lead === undefined ? value : { text: linkText(value), lead };
  }
  passOver(value, path, take);
  return value === null ? "" : JSON.stringify(value);
}

/** passOver takes the property links value holds, at path and under it, as an object or a list shows none. */
function passOver(value: unknown, path: string, take: Take) {
  if (typeof value === "string") {
    take(value, path);
  } else if (Array.isArray(value)) {
    value.forEach((item: unknown, at) => passOver(item, `${path}.${at}`, take));
  } else if (typeof value === "object" && value !== null) {
    for (const [key, item] of Object.entries(value)) {
      passOver(item, `${path}.${key}`, take);
    }
  }
}

/**
 * linkLike tells whether a string has a property link's shape (fixtures'
 * rule 10), which tells whose a link is where two values share its path:
 * one wikilink with a target, or one Markdown link with a target, not to
 * an address elsewhere, with no space around it. The server parses the
 * value as the body; the Markdown link's shape here is near it, not it
 * (accepted): its text holds no bracket but an escaped one, its
 * destination in <> or with no space, and parentheses in it a pair deep
 * at most. What the frontmatter's YAML wrote (an alias's value, a block on
 * several lines) is not known here either (accepted).
 */
function linkLike(value: string): boolean {
  // Both shapes span the whole value: one with a space around it is neither.
  const wikilink = /^\[\[([^[\]\n]*)\]\]$/.exec(value);
  if (wikilink !== null) {
    const inner = wikilink[1] ?? "";
    const bar = inner.indexOf("|");
    const link = bar === -1 ? inner : inner.slice(0, bar);
    return trimmed(link.split("#")[0]?.replace(/\\$/, "") ?? "") !== "";
  }
  const markdown = markdownLink.exec(value);
  const destination = markdown?.[1] ?? markdown?.[2] ?? "";
  return destination.split("#")[0] !== "" && !/^(?:[A-Za-z][A-Za-z0-9+.-]*:|\/\/)/.test(destination);
}

/**
 * markdownLink is one Markdown link, a whole value: its text; its
 * destination, in <> (the first group) or not (the second, not empty: the
 * spaces around it go to no two parts, which would cost a time the
 * square of their length), after spaces maybe; a title maybe, in quotes or
 * parentheses after a space.
 */
const markdownLink =
  /^\[(?:[^[\]\\]|\\.)*\]\([ \t\n]*(?:<((?:[^<>\n\\]|\\.)*)>|(?!<)((?:[^ \t\n()\\]|\\.|\([^ \t\n()]*\))+))(?:[ \t\n]+(?:"[^"]*"|'[^']*'|\([^()]*\)))?[ \t\n]*\)$/s;

/**
 * show is a value shown: its text, or its link, leading to its page or styled as one to none, or to an attachment's
 * content: one the browser shows in a tab of its own, which leaves the page open, as it says unseen; any other
 * downloaded; its text alone when the attachment has no address.
 */
function show(shown: Shown, href: (id: string) => string, newTab: string): ReactNode {
  if (typeof shown === "string") {
    return shown;
  }
  if (typeof shown.lead === "object" && shown.lead !== null) {
    if (shown.lead.asset === null) {
      return shown.text;
    }
    const { asset, inline } = shown.lead;
    return inline ? (
      <a href={asset} target="_blank" rel="noopener noreferrer" className="underline underline-offset-4">
        {shown.text} <span className="sr-only">{newTab}</span>
      </a>
    ) : (
      <a href={asset} download className="underline underline-offset-4">
        {shown.text}
      </a>
    );
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

/**
 * trimmed is text without the spaces and tabs at its ends, as a wikilink's parts are read; by going in from each end,
 * as a pattern for the last would try every run of them and cost a time the square of its length.
 */
function trimmed(text: string): string {
  let start = 0;
  let end = text.length;
  while (start < end && (text[start] === " " || text[start] === "\t")) {
    start++;
  }
  while (end > start && (text[end - 1] === " " || text[end - 1] === "\t")) {
    end--;
  }
  return text.slice(start, end);
}
