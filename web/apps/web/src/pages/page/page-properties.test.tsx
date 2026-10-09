import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, onTestFinished, test, vi } from "vitest";

import { json, problem } from "../../test/fakes";
import { pageEditor } from "../../test/page-editor";
import { readAgain, section, shownPanel } from "../../test/page-panel";
import { guide, install, linux, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The right column's properties (M6/P7 design 10).

/** properties is what the properties show, in their list named so: each key, and its value's text. */
function properties(): string[][] {
  return [...within(section("Properties")).getByLabelText("Properties").querySelectorAll("dl > div")].map((row) => [
    row.querySelector("dt")?.textContent ?? "",
    row.querySelector("dd")?.textContent ?? "",
  ]);
}

test("the properties show each key and its value; a property link its text, leading to its page, or to none, styled so", async () => {
  const user = userEvent.setup();
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "status", value: "draft" },
      { key: "count", value: 3 },
      { key: "done", value: false },
      { key: "empty", value: null },
      { key: "meta", value: { a: 1 } },
      { key: "up", value: "[[Guide| the guide ]]" },
      {
        key: "related",
        value: ["[[ Notes ]]", "[Linux *x*](Linux)", "[[Gone#Part]]", "[[Guide # Intro]]", "[[#Top]]", ["x"]],
      },
      // A table's \\| ends the target; a title may hold ](.
      { key: "escaped", value: "[[Guide\\|]]" },
      { key: "titled", value: '[Linux](Linux "a](b")' },
    ],
    links: [
      { key: "up", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "related.0", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "related.1", node_id: linux.id, kind: "page", url: null, inline: null },
      { key: "related.2", node_id: null, kind: null, url: null, inline: null },
      { key: "related.3", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "escaped", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "titled", node_id: linux.id, kind: "page", url: null, inline: null },
    ],
  });
  const { router } = renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "the guide" });
  expect(properties()).toEqual([
    ["status", "draft"],
    ["count", "3"],
    ["done", "false"],
    ["empty", ""],
    ["meta", '{"a":1}'],
    ["up", "the guide"],
    ["related", 'NotesLinux *x*Gone > PartGuide > Intro[[#Top]]["x"]'],
    ["escaped", "Guide"],
    ["titled", "Linux"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["the guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux *x*", pagePath(linux.id)],
    ["Guide > Intro", pagePath(guide.id)],
    ["Guide", pagePath(guide.id)],
    ["Linux", pagePath(linux.id)],
  ]);
  expect(within(section("Properties")).getByText("Gone > Part").className).toContain("decoration-dashed");

  await user.click(links[1] as HTMLElement);
  const heading = await screen.findByRole("heading", { level: 1, name: "Notes" });
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
  await waitFor(() => expect(document.activeElement).toBe(heading));
});

test("a property link to an attachment the browser shows opens it in a tab of its own, as it says unseen; any other downloads it; one without an address shows its text alone", async () => {
  const server = pageServer();
  const address = "/api/v0/assets/x/content?b=y&e=1&s=z";
  const archive = "/api/v0/assets/z/content?b=y&e=1&s=z";
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "cover", value: "[[x.png|the cover]]" },
      { key: "file", value: "[[a.zip]]" },
      { key: "gone", value: "[t](y.png)" },
      { key: "up", value: "[[Guide]]" },
    ],
    links: [
      { key: "cover", node_id: notes.id, kind: "asset", url: address, inline: true },
      { key: "file", node_id: install.id, kind: "asset", url: archive, inline: false },
      { key: "gone", node_id: linux.id, kind: "asset", url: null, inline: null },
      { key: "up", node_id: guide.id, kind: "page", url: null, inline: null },
    ],
  });
  const { router } = renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  expect(properties()).toEqual([
    ["cover", "the cover (opens in a new tab)"],
    ["file", "a.zip"],
    ["gone", "t"],
    ["up", "Guide"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["the cover (opens in a new tab)", address],
    ["a.zip", archive],
    ["Guide", pagePath(guide.id)],
  ]);
  expect(within(section("Properties")).getByRole("link", { name: "the cover (opens in a new tab)" })).toBe(links[0]);
  expect(links[0]?.querySelector(".sr-only")?.textContent).toBe("(opens in a new tab)");
  expect(links[0]?.getAttribute("target")).toBe("_blank");
  expect(links[0]?.getAttribute("rel")).toBe("noopener noreferrer");
  expect(links[0]?.hasAttribute("download")).toBe(false);
  expect(links[1]?.getAttribute("download")).toBe("");
  expect(links[1]?.hasAttribute("target")).toBe(false);
  const text = within(section("Properties")).getByText("t");
  expect(text.closest("a")).toBeNull();
  expect(text.className).not.toContain("decoration-dashed");
  await userEvent.click(links[0] as HTMLElement);
  expect(router.state.location.pathname).toBe(pagePath(install.id));
});

test("properties that could not be read say why, and are read again on Try again", async () => {
  let fail = true;
  const server = pageServer({
    answers: {
      "GET /api/v0/pages/*/properties": () =>
        fail
          ? problem(500, "internal")
          : json({ valid: true, properties: [{ key: "status", value: "draft" }], links: [], assets_expire_at: null }),
    },
  });
  renderApp(pagePath(install.id), server.app);

  const retry = await within(await shownPanel()).findByRole("button", { name: "Try again" });
  expect(within(section("Properties")).getByRole("alert").textContent).not.toBe("");
  fail = false;
  await userEvent.click(retry);
  await waitFor(() => expect(properties()).toEqual([["status", "draft"]]));
  expect(within(section("Properties")).queryByRole("alert")).toBeNull();
});

test("properties read again show what was written meanwhile, elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onTestFinished(() => void vi.useRealTimers());
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    properties: [{ key: "status", value: "draft" }],
    links: [],
    assets_expire_at: null,
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByText("draft");

  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "status", value: "done" },
      { key: "up", value: "[[Guide]]" },
    ],
    links: [{ key: "up", node_id: guide.id, kind: "page", url: null, inline: null }],
  });
  await readAgain();
  await waitFor(() =>
    expect(properties()).toEqual([
      ["status", "done"],
      ["up", "Guide"],
    ])
  );
  expect(within(section("Properties")).getByRole("link", { name: "Guide" }).getAttribute("href")).toBe(
    pagePath(guide.id)
  );
});

test("a frontmatter that is not valid says so, as one without properties does", async () => {
  const server = pageServer();
  server.properties.set(install.id, { valid: false, properties: [], links: [], assets_expire_at: null });
  renderApp(pagePath(install.id), server.app);
  expect(
    await within(await shownPanel()).findByText("The page's frontmatter is not valid: it has no properties.")
  ).toBeTruthy();

  renderApp(pagePath(guide.id), pageServer().app);
  expect(await screen.findByText("No properties.")).toBeTruthy();
});

test("while the page is edited its backlinks and properties are shown", async () => {
  const server = pageServer({ role: "editor" });
  server.backlinks.set(install.id, [{ data: [{ id: guide.id, count: 1, contexts: [] }], next_cursor: null }]);
  server.properties.set(install.id, {
    valid: true,
    properties: [{ key: "status", value: "draft" }],
    links: [],
    assets_expire_at: null,
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByRole("link", { name: "Guide" });

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  await pageEditor();
  expect(within(section("Backlinks")).getByRole("link", { name: "Guide" })).toBeTruthy();
  expect(properties()).toEqual([["status", "draft"]]);
});

test("property links at a path two values share are theirs in turn, in the order written, those an object holds too", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "rel.0", value: "[[Guide]]" },
      { key: "rel", value: ["[[Notes]]", "[draft]"] },
      { key: "a", value: { b: "[[Missing]]" } },
      { key: "a.b", value: "[[Linux]]" },
      // A value that is no link takes none: the next value at its path has it.
      { key: "x.0", value: "plain" },
      { key: "x", value: ["[[Guide]]"] },
    ],
    links: [
      { key: "rel.0", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "rel.0", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "a.b", node_id: null, kind: null, url: null, inline: null },
      { key: "a.b", node_id: linux.id, kind: "page", url: null, inline: null },
      { key: "x.0", node_id: guide.id, kind: "page", url: null, inline: null },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Linux" });
  expect(properties()).toEqual([
    ["rel.0", "Guide"],
    ["rel", "Notes[draft]"],
    ["a", '{"b":"[[Missing]]"}'],
    ["a.b", "Linux"],
    ["x.0", "plain"],
    ["x", "Guide"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
    ["Guide", pagePath(guide.id)],
  ]);
});

test("a value that is no link takes none of its path's: an anchor alone, a bracketed word, an address elsewhere, a space around; a list's lists pass theirs", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "a.0", value: "[[#Top]]" },
      { key: "a", value: ["[[Guide]]"] },
      { key: "b.0", value: "[WIP]" },
      { key: "b", value: ["[[Notes]]"] },
      { key: "c.0", value: "[site](https://example.com)" },
      { key: "c", value: ["[[Linux]]"] },
      { key: "d.0", value: " [[Guide]]" },
      { key: "d", value: ["[[Notes]]"] },
      { key: "e", value: [["[[Guide]]"]] },
      { key: "e.0.0", value: "[[Linux]]" },
    ],
    links: [
      { key: "a.0", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "b.0", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "c.0", node_id: linux.id, kind: "page", url: null, inline: null },
      { key: "d.0", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "e.0.0", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "e.0.0", node_id: linux.id, kind: "page", url: null, inline: null },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  expect(properties()).toEqual([
    ["a.0", "[[#Top]]"],
    ["a", "Guide"],
    ["b.0", "[WIP]"],
    ["b", "Notes"],
    ["c.0", "[site](https://example.com)"],
    ["c", "Linux"],
    ["d.0", " [[Guide]]"],
    ["d", "Notes"],
    ["e", '["[[Guide]]"]'],
    ["e.0.0", "Linux"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Guide", pagePath(guide.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
    ["Notes", pagePath(notes.id)],
    ["Linux", pagePath(linux.id)],
  ]);
});

test("which values take their path's link is the server's shape of one: a target, no address elsewhere, one link whole", async () => {
  const notLinks = [
    "[[ ]]",
    "[[\\|b]]",
    "[[a]]x",
    "[[a]b]]",
    "[a](#x)",
    "[a]( #x)",
    "[a](//x.test)",
    "[site](<https://x.test>)",
    "[a](<Guide)",
    "[a](Hub) [b](Other)",
    "[Install](Install Guide)",
  ];
  // An address in <> with a space, a title, parentheses a pair deep.
  const links = ["[Install](<Install Guide>)", '[a](Linux "Linux")', "[a](Notes(1))"];
  const server = pageServer();
  // Each value at a path a list's first item shares: a value that is no link leaves the item its link.
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      ...notLinks.flatMap((value, at) => [
        { key: `n${at.toString()}.0`, value },
        { key: `n${at.toString()}`, value: ["[[Guide]]"] },
      ]),
      ...links.flatMap((value, at) => [
        { key: `l${at.toString()}.0`, value },
        { key: `l${at.toString()}`, value: ["[[Notes]]"] },
      ]),
    ],
    links: [
      ...notLinks.map((_, at) => ({
        key: `n${at.toString()}.0`,
        node_id: guide.id,
        kind: "page" as const,
        url: null,
        inline: null,
      })),
      ...links.flatMap((_, at) => [
        { key: `l${at.toString()}.0`, node_id: linux.id, kind: "page" as const, url: null, inline: null },
        { key: `l${at.toString()}.0`, node_id: notes.id, kind: "page" as const, url: null, inline: null },
      ]),
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Install" });
  expect(properties()).toEqual([
    ...notLinks.flatMap((value, at) => [
      [`n${at.toString()}.0`, value],
      [`n${at.toString()}`, "Guide"],
    ]),
    ["l0.0", "Install"],
    ["l0", "Notes"],
    ["l1.0", "a"],
    ["l1", "Notes"],
    ["l2.0", "a"],
    ["l2", "Notes"],
  ]);
  const led = within(section("Properties"))
    .getAllByRole("link")
    .map((link) => link.getAttribute("href"));
  expect(led).toEqual([
    ...notLinks.map(() => pagePath(guide.id)),
    ...links.flatMap(() => [pagePath(linux.id), pagePath(notes.id)]),
  ]);
});

test("a value at a path of its own has the path's link, whatever its shape: the server pairs them", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "spec", value: "[Spec [v2]](Guide)" },
      { key: "code", value: "[`a[0]`](Notes)" },
      { key: "titled", value: '[T](Linux "Ti\\"tle")' },
      { key: "plain", value: "[WIP]" },
    ],
    links: [
      { key: "spec", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "code", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "titled", node_id: linux.id, kind: "page", url: null, inline: null },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "T" });
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => [link.textContent, link.getAttribute("href")])).toEqual([
    ["Spec [v2]", pagePath(guide.id)],
    ["`a[0]`", pagePath(notes.id)],
    ["T", pagePath(linux.id)],
  ]);
  expect(properties().at(-1)).toEqual(["plain", "[WIP]"]);
});

test("a property costs a time as long as it: a long value that is no link at a path two share, a wikilink with long parts, many strings at long paths", async () => {
  const spaces = " ".repeat(1 << 18);
  // Strings at paths as long as V8 hashes by their length alone.
  const many = Array.from({ length: 5_000 }, () => "x");
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: "a.0", value: `[a](${spaces}x y)` },
      { key: "a", value: ["[[Guide]]"] },
      { key: "b", value: `[[Notes${spaces}x]]` },
      { key: "k".repeat(17_000), value: [many] },
    ],
    links: [
      { key: "a.0", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "b", node_id: notes.id, kind: "page", url: null, inline: null },
    ],
  });
  const started = performance.now();
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Guide" });
  expect(performance.now() - started).toBeLessThan(3_000);
});

test("a path's strings are counted alone, in objects and lists' lists too, numbers not; a wikilink's parts lose their tabs", async () => {
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      // A number at the path of a list's item: the item is the only string there, whatever its shape.
      { key: "a.0", value: 5 },
      { key: "a", value: ["[Spec [v2]](Guide)"] },
      // An object's string and a key's at one path, a list's list's and a key's: the shape tells.
      { key: "m", value: { x: "plain" } },
      { key: "m.x", value: "[[Notes]]" },
      { key: "e", value: [["plain"]] },
      { key: "e.0.0", value: "[[Linux]]" },
      { key: "t", value: "[[\tGuide\t]]" },
    ],
    links: [
      { key: "a.0", node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "m.x", node_id: notes.id, kind: "page", url: null, inline: null },
      { key: "e.0.0", node_id: linux.id, kind: "page", url: null, inline: null },
      { key: "t", node_id: guide.id, kind: "page", url: null, inline: null },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Spec [v2]" });
  expect(properties()).toEqual([
    ["a.0", "5"],
    ["a", "Spec [v2]"],
    ["m", '{"x":"plain"}'],
    ["m.x", "Notes"],
    ["e", '["plain"]'],
    ["e.0.0", "Linux"],
    ["t", "Guide"],
  ]);
  const links = within(section("Properties")).getAllByRole("link");
  expect(links.map((link) => link.getAttribute("href"))).toEqual([
    pagePath(guide.id),
    pagePath(notes.id),
    pagePath(linux.id),
    pagePath(guide.id),
  ]);
});

test("a property link at a path longer than 1,024 characters shows as its text; one at 1,024 leads to its page", async () => {
  const server = pageServer();
  const [longest, longer] = ["q".repeat(1_024), "p".repeat(1_025)];
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: longest, value: "[[Notes]]" },
      { key: longer, value: "[[Guide]]" },
    ],
    links: [
      { key: longest, node_id: notes.id, kind: "page", url: null, inline: null },
      { key: longer, node_id: guide.id, kind: "page", url: null, inline: null },
    ],
  });
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Notes" });
  expect(properties()).toEqual([
    [longest, "Notes"],
    [longer, "[[Guide]]"],
  ]);
});

test("the properties are worked out once for each answer, not as the column renders again: the edit entered (keys typed in it render none)", async () => {
  const server = pageServer({ role: "editor" });
  server.properties.set(install.id, {
    valid: true,
    properties: [{ key: "o", value: { "nw-once": 1 } }],
    links: [],
    assets_expire_at: null,
  });
  renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByText('{"nw-once":1}');
  const stringify = vi.spyOn(JSON, "stringify");
  onTestFinished(() => stringify.mockRestore());

  fireEvent.keyDown(document.activeElement ?? document.body, { key: "e", ctrlKey: true });
  const { type } = await pageEditor();
  for (const key of "abc") {
    act(() => type(key));
  }
  await act(async () => {});
  expect(properties()).toEqual([["o", '{"nw-once":1}']]);
  expect(
    stringify.mock.calls.filter(([value]) => typeof value === "object" && value !== null && "nw-once" in value)
  ).toEqual([]);
});

test("no path past 1,024 is a map's or a set's key: a browser costs the square of their number for strings that long", async () => {
  const long = "k".repeat(1_100);
  const server = pageServer();
  server.properties.set(install.id, {
    valid: true,
    assets_expire_at: null,
    properties: [
      { key: long, value: ["[[Guide]]", "x"] },
      { key: "short", value: "[[Notes]]" },
    ],
    links: [
      { key: `${long}.0`, node_id: guide.id, kind: "page", url: null, inline: null },
      { key: "short", node_id: notes.id, kind: "page", url: null, inline: null },
    ],
  });
  const keyed = [
    vi.spyOn(Map.prototype, "get"),
    vi.spyOn(Map.prototype, "has"),
    vi.spyOn(Map.prototype, "set"),
    vi.spyOn(Set.prototype, "has"),
    vi.spyOn(Set.prototype, "add"),
  ];
  onTestFinished(() => void vi.restoreAllMocks());
  renderApp(pagePath(install.id), server.app);

  await within(await shownPanel()).findByRole("link", { name: "Notes" });
  const paths = keyed.flatMap((spy) => spy.mock.calls.map(([key]: unknown[]) => key));
  expect(paths.filter((key) => typeof key === "string" && key.startsWith(`${long}.`))).toEqual([]);
  expect(properties()).toEqual([
    [long, "[[Guide]]x"],
    ["short", "Notes"],
  ]);
});

/** expiring is a page's properties whose status is status, with a link to an attachment whose address expires at expires. */
function expiring(status: string, expires: string | null) {
  return {
    valid: true,
    assets_expire_at: expires,
    properties: [
      { key: "cover", value: "[[x.png]]" },
      { key: "status", value: status },
    ],
    links: [
      { key: "cover", node_id: notes.id, kind: "asset" as const, url: "/api/v0/assets/x/content?e=1", inline: true },
    ],
  };
}

/** inMinutes is the time minutes from now, as the server writes it. */
const inMinutes = (minutes: number) => new Date(Date.now() + minutes * 60_000).toISOString();

test("properties are read again a minute before their attachments' addresses expire; the cache's, expired, are none until read again", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  onTestFinished(() => void vi.useRealTimers());
  const server = pageServer();
  server.properties.set(install.id, expiring("draft", inMinutes(10)));
  const { router } = renderApp(pagePath(install.id), server.app);
  await within(await shownPanel()).findByText("draft");
  const reads = () => server.sent.filter((line) => line === `GET properties ${install.id}`).length;
  expect(reads()).toBe(1);

  server.properties.set(install.id, expiring("done", inMinutes(70)));
  await act(() => vi.advanceTimersByTimeAsync(8.9 * 60_000));
  expect(reads()).toBe(1);
  await act(() => vi.advanceTimersByTimeAsync(0.2 * 60_000));
  await within(section("Properties")).findByText("done");
  expect(reads()).toBe(2);

  await act(() => router.navigate(pagePath(guide.id)));
  await within(section("Properties")).findByText("No properties.");
  await act(() => vi.advanceTimersByTimeAsync(80 * 60_000));
  server.properties.set(install.id, expiring("final", null));
  await act(() => router.navigate(pagePath(install.id)));
  expect(within(section("Properties")).queryByText("done")).toBeNull();
  await within(section("Properties")).findByText("final");
  await act(() => vi.advanceTimersByTimeAsync(3 * 60 * 60_000));
  expect(reads()).toBe(3);
});
