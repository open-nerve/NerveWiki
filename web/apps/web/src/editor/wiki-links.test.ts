import { expect, test } from "vitest";

import { shownOf, wikiLinks } from "./wiki-links";

// The wiki links the editor finds, as the server reads them (server/internal/platform/markdown/obsidian/wikilink.go;
// M13/P4/S3, decision 907, review A's P4S3A-N2).

/** links is what wikiLinks finds in text: each link's text and what the reading view shows of it. */
const links = (text: string) => wikiLinks(text).map((link) => [text.slice(link.from, link.to), shownOf(link)]);

test("a wiki link shows its display text, or its target and anchor as Obsidian writes them; an attachment's embed its caption, but a size; a page's its target", () => {
  expect(links("[[Page]] [[Page|alias]] [[Page#Sec]] [[#Sec]] [[Page#Sec|there]]")).toEqual([
    ["[[Page]]", "Page"],
    ["[[Page|alias]]", "alias"],
    ["[[Page#Sec]]", "Page > Sec"],
    ["[[#Sec]]", "Sec"],
    ["[[Page#Sec|there]]", "there"],
  ]);
  expect(
    links("![[a.png]] ![[a.png|300]] ![[a.png|20x30]] ![[a.png|Diagram]] ![[a.png|Diagram|300]] ![[Page#Sec]]")
  ).toEqual([
    ["![[a.png]]", "a.png"],
    ["![[a.png|300]]", "a.png"],
    ["![[a.png|20x30]]", "a.png"],
    ["![[a.png|Diagram]]", "Diagram"],
    ["![[a.png|Diagram|300]]", "Diagram"],
    ["![[Page#Sec]]", "Page > Sec"],
  ]);
  // The server shows a page's embed by its target (wikilink.go's shown): one with no extension but ".md" is a page's.
  expect(links("![[Page|alias]] ![[Page|cap|300]] ![[Notes/Page.md|x]] ![[v1.2/Page|y]] ![[Doc.PDF|Spec]]")).toEqual([
    ["![[Page|alias]]", "Page"],
    ["![[Page|cap|300]]", "Page"],
    ["![[Notes/Page.md|x]]", "Notes/Page.md"],
    ["![[v1.2/Page|y]]", "v1.2/Page"],
    ["![[Doc.PDF|Spec]]", "Spec"],
  ]);
});

test("none is a wiki link whose first bracket is escaped, nor one with neither a target nor an anchor, nor one with a bracket inside", () => {
  expect(links("\\[[a]] \\\\[[b]] \\![[c]] [[|x]] [[ ]] [[#]] [[ # |y]] [[a]b]] [[d[e]]")).toEqual([
    ["[[b]]", "b"],
    // An escaped "!" is text: the link after it is a link, not an embed.
    ["[[c]]", "c"],
  ]);
});

test("a wiki link's target, anchor and bar are where they are written, trimmed; a table's \\| is its bar", () => {
  const text = "x [[ Page # Sec | alias ]] [[P\\|al]]";
  const [spaced, piped] = wikiLinks(text, 10);
  const at = (range: { from: number; to: number } | undefined) =>
    range === undefined ? undefined : text.slice(range.from - 10, range.to - 10);
  expect([spaced?.from, spaced?.to, spaced?.target, spaced?.anchor, spaced?.display]).toEqual([
    12,
    36,
    "Page",
    "Sec",
    "alias",
  ]);
  expect([at(spaced?.targetAt), at(spaced?.anchorAt), at(spaced?.bar)]).toEqual(["Page", "Sec", "|"]);
  expect(spaced?.hash === undefined ? undefined : text[spaced.hash - 10]).toBe("#");
  expect([piped?.target, piped?.display, at(piped?.targetAt), at(piped?.bar), piped?.hash]).toEqual([
    "P",
    "al",
    "P",
    "\\|",
    undefined,
  ]);
});
