import { readFileSync } from "node:fs";
import { URL as FileURL } from "node:url";
import { expect, onTestFinished, test } from "vitest";

import { addressesOnly, findByKey, focusKeyOf, patchBlocks, resign, template } from "./patch";

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
  ["one fewer address, the last", (html: string) => html.replace(/<p>The figures are in [^\n]*<\/p>\n/, "")],
])("resign changes nothing when the article's addresses are not the view's: %s", (_, live) => {
  const first = signed("1795003200", "A".repeat(22));
  const { article } = written(live(first));
  const html = article.innerHTML;

  expect(resign(article, template(first), template(signed("1795006800", "B".repeat(22))))).toBe(false);

  expect(article.innerHTML).toBe(html);
});

/** blocks are article's top-level nodes. */
const blocks = (article: Element) => [...article.childNodes];

/** taken watches article's subtree: it answers the nodes taken out of it since. */
function taken(article: Element): () => Node[] {
  const watching = new MutationObserver(() => undefined);
  watching.observe(article, { childList: true, subtree: true });
  onTestFinished(() => watching.disconnect());
  return () => {
    const out: Node[] = [];
    for (const record of watching.takeRecords()) {
      out.push(...record.removedNodes);
    }
    return out;
  };
}

// M12/P2/S4, 8.2: the ends alike are taken from the end first.
test("patchBlocks keeps the heading the reader was at as a section of its name is put before it: its id is the new one's", () => {
  const old = '<h2 id="nw-notes">Notes</h2>\n<p>one</p>\n';
  const after = '<h2 id="nw-notes">Notes</h2>\n<p>zero</p>\n<h2 id="nw-notes-1">Notes</h2>\n<p>one</p>\n';
  const { article } = written(old);
  const [heading, , one] = blocks(article);
  const out = taken(article);

  const placed = patchBlocks(article, template(old), template(after));

  expect(article.innerHTML).toBe(after);
  expect(blocks(article)[4]).toBe(heading);
  expect(blocks(article)[6]).toBe(one);
  expect((heading as Element).id).toBe("nw-notes-1");
  expect(out()).toEqual([]);
  expect([0, 2].map((at) => placed?.(at))).toEqual([4, 6]);
});

// M12/P2/S4, P2-21: between the ends, each block takes the first of its kind past the last taken.
test("patchBlocks keeps the blocks alike between ends that changed, takes out the others, puts in the new, and moves none", () => {
  const old = "<p>a 1</p>\n<p>same</p>\n<p>gone</p>\n<ul><li>list</li></ul>\n<p>b 1</p>\n";
  const after = "<p>a 2</p>\n<p>same</p>\n<p>new</p>\n<ul><li>list</li></ul>\n<p>b 2</p>\n";
  const { article } = written(old);
  const [first, , same, , gone, , list, , last] = blocks(article);
  const out = taken(article);

  const placed = patchBlocks(article, template(old), template(after));

  expect(article.innerHTML).toBe(after);
  expect([blocks(article)[2], blocks(article)[6]]).toEqual(
    [same, list].map((node) => expect.toSatisfy((x) => x === node))
  );
  expect(out().filter((node) => node.nodeType === Node.ELEMENT_NODE)).toEqual(
    [first, gone, last].map((node) => expect.toSatisfy((x) => x === node))
  );
  // Each taken out is where the one put in its place is.
  expect([0, 2, 4, 6, 8].map((at) => placed?.(at))).toEqual([0, 2, 4, 6, 8]);
});

test("patchBlocks places a block taken out at the next kept when none is put in its place, at the last block when none follows", () => {
  const old = "<p>keep</p>\n<p>gone</p>\n<p>tail</p>\n<p>end</p>\n";
  const after = "<p>keep</p>\n<p>tail</p>\n";
  const { article } = written(old);

  const placed = patchBlocks(article, template(old), template(after));

  expect(article.innerHTML).toBe(after);
  expect([placed?.(2), placed?.(6)]).toEqual([2, 2]);
});

// M12/P2/S4, 4.4.3's step 3 (review A's C1-M1): where a block was taken out, the others put in, nearest first.
test("patchBlocks tells of a block taken out the others put in where it was, nearest first, the later of two as near", () => {
  const old = "<p>a</p><p>b</p><p>c</p><p>d</p><p>z</p>";
  const after = "<p>a</p><p>B</p><p>C</p><p>D</p><p>z</p>";
  const { article } = written(old);

  const placed = patchBlocks(article, template(old), template(after));

  expect([1, 2, 3].map((at) => placed?.(at))).toEqual([1, 2, 3]);
  expect([1, 2, 3].map((at) => placed?.near?.(at))).toEqual([
    [2, 3],
    [3, 1],
    [2, 1],
  ]);
  expect([0, 4].map((at) => placed?.near?.(at))).toEqual([[], []]);
});

// M12/P2/S4, 4.4.2 step 4: from the HTML before to the HTML after, not from what the reader has.
test("patchBlocks gives a block kept the new ids and addresses, and leaves what the reader changed: a callout opened, a media begun", () => {
  const block = (id: string, e: string, s: string) =>
    `<details id="${id}"><summary>Clip</summary><video class="nw-asset" src="${address(clip, { e, s })}"></video>` +
    `<img class="nw-asset" src="${address(chart, { e, s })}"></details>\n`;
  const old = `<p>a</p>\n${block("nw-x", "1795003200", "A".repeat(22))}`;
  const after = `<p>b</p>\n${block("nw-x-1", "1795006800", "B".repeat(22))}`;
  const { article } = written(old);
  const details = article.querySelector("details") as HTMLDetailsElement;
  details.open = true;
  const video = article.querySelector("video") as HTMLVideoElement;
  Object.defineProperty(video, "paused", { configurable: true, value: false });

  patchBlocks(article, template(old), template(after));

  expect(article.querySelector("details")).toBe(details);
  expect([details.id, details.open]).toEqual(["nw-x-1", true]);
  expect(video.getAttribute("src")).toBe(plain(address(clip)));
  expect(article.querySelector("img")?.getAttribute("src")).toBe(
    plain(address(chart, { e: "1795006800", s: "B".repeat(22) }))
  );
});

test("patchBlocks puts a block in anew whose ids changed and whose nodes are not the HTML's: one left in it", () => {
  const old = '<p>a</p>\n<div id="nw-c"><p>x = 1</p></div>\n';
  const after = '<p>b</p>\n<div id="nw-c-1"><p>x = 1</p></div>\n';
  const { article } = written(old);
  article.querySelector("div p")?.append(Object.assign(document.createElement("span"), { textContent: "left" }));
  const div = article.querySelector("div");

  patchBlocks(article, template(old), template(after));

  expect(article.innerHTML).toBe(after);
  expect(article.querySelector("div")).not.toBe(div);
});

// M12/P2/S4 (review A's M2): a callout of a screenshot and a code block, the code coloured, its addresses changed as
// another wrote after the hour: the code's nodes are the highlighting's, and none has an id or an address.
test("patchBlocks keeps a block whose code is coloured, giving it the new ids and addresses: a callout opened stays", () => {
  const block = (id: string, e: string, s: string) =>
    `<details id="${id}"><summary>Steps</summary><p><img class="nw-asset" src="${address(chart, { e, s })}"></p>` +
    `<pre><code class="language-go">x := 1</code></pre></details>\n`;
  const old = `<p>a</p>\n${block("nw-x", "1795003200", "A".repeat(22))}`;
  const after = `<p>b</p>\n${block("nw-x-1", "1795006800", "B".repeat(22))}`;
  const { article } = written(old);
  const details = article.querySelector("details") as HTMLDetailsElement;
  details.open = true;
  const code = article.querySelector("code") as HTMLElement;
  code.innerHTML = '<span class="hljs-attr">x</span> := <span class="hljs-number">1</span>';

  patchBlocks(article, template(old), template(after));

  expect(article.querySelector("details")).toBe(details);
  expect([details.id, details.open]).toEqual(["nw-x-1", true]);
  expect(article.querySelector("img")?.getAttribute("src")).toBe(
    plain(address(chart, { e: "1795006800", s: "B".repeat(22) }))
  );
  expect(code.querySelectorAll("span")).toHaveLength(2);
});

// M12/P2/S4, 4.4.2 step 1: the article's blocks not the view's, the HTML is written whole.
test("patchBlocks changes nothing and answers none when the article's top-level nodes are not the HTML's", () => {
  const old = "<p>a</p>\n<p>b</p>\n";
  const { article } = written(`${old}<div>left by an enhancement</div>`);
  const html = article.innerHTML;

  expect(patchBlocks(article, template(old), template("<p>a</p>\n<p>b</p>\n<p>c</p>\n"))).toBeUndefined();
  expect(article.innerHTML).toBe(html);
});

/** keyedView is a view of elements of each key, its ids ending with n, its address signed anew for any n. */
const keyedView = (n: string) =>
  `<h2 id="nw-a${n}">A</h2><p><a href="${address(chart, { e: n === "" ? "1795003200" : "1795006800" })}">c</a>` +
  `<a class="nw-unresolved" data-nw-target="T">t</a></p><details><summary>S</summary></details>` +
  `<audio class="nw-asset" src="${address(talk)}"></audio><h2 id="nw-a-1${n}">A</h2><sup id="fn"></sup>`;

// M12/P2/S4, 4.4.3 step 3.
test("an element is found again by its key: a heading by its text, not its id, a link by its address and text, a callout's summary, a media, an id", () => {
  const { article: one } = written(keyedView(""));
  const { article: two } = written(keyedView("x"));
  const pairs = [...one.querySelectorAll("h2, a, summary, audio, sup")].map((element) => {
    const key = focusKeyOf(one, element);
    return key && findByKey(two, key)?.outerHTML;
  });

  expect(pairs).toEqual([...two.querySelectorAll("h2, a, summary, audio, sup")].map((element) => element.outerHTML));
  expect(focusKeyOf(one, one.querySelector("p") as Element)).toBeUndefined();
});
