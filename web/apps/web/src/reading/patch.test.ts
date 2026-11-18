import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { expect, onTestFinished, test } from "vitest";

import { addressesOnly, resign, template } from "./patch";

// The reading view's HTML changed in place (M12/P2/S4, P2 design 4.4.2).

type SignedView = { at: string; html: string; assets_expire_at: string };

// The server's reading view of one page at two times an hour apart, which the asset module's test checks against what
// the server writes, byte for byte (P2-45): the spelling of an address is the server's, not this test's idea of it.
const sample = JSON.parse(
  readFileSync(new FileURL("../../../../../testdata/m12/signed-view.json", import.meta.url), "utf8")
) as { views: [SignedView, SignedView] };

/** address is an attachment's content's address as the server writes it, of node and blob, with e, s and d. */
function address(
  node: string,
  { b = "0199a2b4-0000-7000-8000-0000000b0001", e = "1795003200", s = "A".repeat(22), d = "" } = {}
) {
  return `/api/v0/assets/${node}/content?b=${b}&amp;e=${e}&amp;s=${s}${d}`;
}

const chart = "0199a2b4-0000-7000-8000-0000000a0001";

/** view is a reading view of an image and a link to a file at their addresses, then a paragraph of text. */
function view(image: string, link: string, text = "tail") {
  return (
    `<p><img class="nw-asset" src="${image}" alt="chart.png"></p>\n` +
    `<p><a class="nw-wikilink nw-asset" href="${link}" download="">report.zip</a></p>\n<p>${text}</p>\n`
  );
}

const before = view(address(chart), address(chart));

test("the sample's two views, signed an hour apart, are alike but for their addresses' expiry and signature", () => {
  const [first, second] = sample.views;
  expect(first.html).not.toBe(second.html);
  expect(addressesOnly(template(first.html), template(second.html))).toBe(true);
  // Another file, or a download in place of the content shown, is a change of the view (U4).
  const blob = first.html.replace("b=0199a2b4-0000-7000-8000-0000000b0002", "b=0199a2b4-0000-7000-8000-0000000b0009");
  const download = first.html.replace(/(src="[^"]*-0000000b0002&amp;e=\d+&amp;s=[\w-]{22})"/, '$1&amp;d=1"');
  expect([blob, download].map((html) => html === first.html)).toEqual([false, false]);
  expect(addressesOnly(template(blob), template(second.html))).toBe(false);
  expect(addressesOnly(template(download), template(second.html))).toBe(false);
});

test.each([
  [
    "their expiry and signature",
    true,
    view(
      address(chart, { e: "1795006800", s: "B".repeat(22) }),
      address(chart, { e: "1795006800", s: "C".repeat(22) })
    ),
  ],
  ["another blob", false, view(address(chart, { b: "0199a2b4-0000-7000-8000-0000000b0009" }), address(chart))],
  ["a download", false, view(address(chart), address(chart, { d: "&amp;d=1" }))],
  ["another attachment", false, view(address("0199a2b4-0000-7000-8000-0000000a0009"), address(chart))],
  ["another text", false, view(address(chart), address(chart), "tail, edited")],
  ["another attribute", false, view(address(chart), address(chart)).replace('alt="chart.png"', 'alt="chart.jpg"')],
] as const)("addressesOnly is true when only the addresses' expiry and signature differ: %s, %s", (_, alike, after) => {
  expect(addressesOnly(template(before), template(after))).toBe(alike);
});

/** quoting is a view whose text writes an address, of expiry e. */
const quoting = (e: string) => view(address(chart), address(chart), `see ${address(chart, { e })}`);

test("an address written in the text is the text: its expiry and signature changed there is a change of the view", () => {
  expect(addressesOnly(template(quoting("1795003200")), template(quoting("1795006800")))).toBe(false);
});

test("what is not the server's spelling of an address is no address: its parts changed are a change", () => {
  // A signature of another length, a query in another order, an address of another route.
  const pairs: [string, string][] = [
    [address(chart, { s: "A".repeat(21) }), address(chart, { e: "1795006800", s: "B".repeat(21) })],
    [
      `/api/v0/assets/${chart}/content?e=1795003200&amp;b=x&amp;s=${"A".repeat(22)}`,
      `/api/v0/assets/${chart}/content?e=1795006800&amp;b=x&amp;s=${"B".repeat(22)}`,
    ],
    [
      `/api/v0/assets/${chart}/thumbnail?b=x&amp;e=1795003200&amp;s=${"A".repeat(22)}`,
      `/api/v0/assets/${chart}/thumbnail?b=x&amp;e=1795006800&amp;s=${"B".repeat(22)}`,
    ],
  ];
  for (const [one, other] of pairs) {
    expect(addressesOnly(template(view(one, address(chart))), template(view(other, address(chart))))).toBe(false);
  }
});

const [talk, clip, report] = ["02", "03", "04"].map((n) => `0199a2b4-0000-7000-8000-0000000a00${n}`) as [
  string,
  string,
  string,
];

/** signed is a view of an image, an audio, a video and a download, their addresses expiring at e, signed s. */
function signed(e: string, s: string) {
  return (
    `<p><img class="nw-asset" src="${address(chart, { e, s })}" alt="chart.png"></p>\n` +
    `<p><audio class="nw-asset" src="${address(talk, { e, s })}" controls=""></audio></p>\n` +
    `<p><video class="nw-asset" src="${address(clip, { e, s })}" controls=""></video></p>\n` +
    `<p>The figures are in <a class="nw-wikilink nw-asset" href="${address(report, { e, s })}" download="">report.zip</a>.</p>\n`
  );
}

/** written is an article in the document with html in it, and every node of it, in order. */
function written(html: string): { article: HTMLElement; nodes: Node[] } {
  const article = document.createElement("article");
  article.innerHTML = html;
  document.body.append(article);
  onTestFinished(() => article.remove());
  const nodes: Node[] = [];
  const walker = document.createTreeWalker(article);
  for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
    nodes.push(node);
  }
  return { article, nodes };
}

/** kept tells whether article has the same nodes as nodes, in order: none taken out, none put in. */
function kept(article: HTMLElement, nodes: Node[]): boolean {
  const walker = document.createTreeWalker(article);
  const now: Node[] = [];
  for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
    now.push(node);
  }
  return now.length === nodes.length && now.every((node, i) => node === nodes[i]);
}

/** plain is an address as an attribute reads, its &amp; a &. */
const plain = (html: string) => html.replaceAll("&amp;", "&");

test("resign gives the links, the images and the media not begun their new addresses, and keeps every node: a begun one keeps its own", () => {
  const first = signed("1795003200", "A".repeat(22));
  const after = signed("1795006800", "B".repeat(22));
  const { article, nodes } = written(first);
  const video = article.querySelector("video") as HTMLVideoElement;
  // Playing: the media's own address, signed anew as it fails (assets.ts).
  Object.defineProperty(video, "paused", { configurable: true, value: false });

  expect(resign(article, template(first), template(after))).toBe(true);

  expect(kept(article, nodes)).toBe(true);
  const at = (selector: string, name: string) => article.querySelector(selector)?.getAttribute(name);
  const e = "1795006800";
  const s = "B".repeat(22);
  expect([at("img", "src"), at("audio", "src"), at("a", "href")]).toEqual(
    [address(chart, { e, s }), address(talk, { e, s }), address(report, { e, s })].map(plain)
  );
  expect(at("video", "src")).toBe(plain(address(clip)));
});

test("resign has the sample's first view the second, every node kept", () => {
  const [first, second] = sample.views;
  const { article, nodes } = written(first.html);

  expect(resign(article, template(first.html), template(second.html))).toBe(true);

  expect(article.innerHTML).toBe(second.html);
  expect(kept(article, nodes)).toBe(true);
});

test.each([
  ["one more address", (html: string) => `${html}<p><img class="nw-asset" src="${address(chart)}"></p>`],
  ["another attachment", (html: string) => html.replace(`/assets/${talk}/`, `/assets/${clip}/`)],
])("resign changes nothing when the article's addresses are not the view's: %s", (_, live) => {
  const first = signed("1795003200", "A".repeat(22));
  const { article } = written(live(first));
  const html = article.innerHTML;

  expect(resign(article, template(first), template(signed("1795006800", "B".repeat(22))))).toBe(false);

  expect(article.innerHTML).toBe(html);
});
