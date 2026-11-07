import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";

import { arrived } from "../../app/arrival";
import { json, notebookJSON, problem } from "../../test/fakes";
import { deferred } from "../../test/health-server";
import {
  firstId,
  indexFacts,
  indexHome,
  indexPath,
  indexRaw,
  indexServer,
  secondId,
  thirdId,
} from "../../test/index-server";
import { guide, notes } from "../../test/page-server";
import { renderApp } from "../../test/render";

function directory() {
  return within(screen.getByRole("region", { name: "Directory pages" }));
}
function titles() {
  return directory()
    .getAllByRole("link")
    .filter((link) => link.getAttribute("href")?.includes("/pages/"))
    .map((link) => link.textContent);
}

// Removing the real route/service wiring fails at this nonempty user result, not a missing import.
test("P4/route: a reader opens the real notebook Directory with three linked pages", async () => {
  const fixture = indexServer();
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("heading", { level: 1, name: "Directory" });
  await screen.findByRole("region", { name: "Directory pages" });
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]);
  expect(directory().getByRole("link", { name: "Alpha" }).getAttribute("href")).toBe(`${indexHome}/pages/${firstId}`);
  expect(directory().getByRole("link", { name: "Bravo" }).getAttribute("href")).toBe(`${indexHome}/pages/${secondId}`);
  expect(directory().getByRole("link", { name: "Charlie" }).getAttribute("href")).toBe(`${indexHome}/pages/${thirdId}`);
  expect(
    screen.getByRole("navigation", { name: "More of Plans" }).querySelector('[aria-current="page"]')?.textContent
  ).toBe("Directory");
  expect(document.title).toContain("Directory");
  expect(fixture.requests).toHaveLength(1);
  expect(screen.getByRole("status", { name: "Directory results" }).textContent).toBe("3 pages shown.");
});

test.each(["standard", "llm_wiki"] as const)("P4/places: Directory is available in %s", async (mode) => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.server.notebook.mode = mode;
  fixture.server.notebook.source_root_id = mode === "llm_wiki" ? indexRaw.id : null;
  fixture.value.mode = mode;
  renderApp(indexHome, fixture.server.app);
  const places = await screen.findByRole("navigation", { name: "More of Plans" });
  await user.click(within(places).getByRole("link", { name: "Directory" }));
  await screen.findByRole("region", { name: "Directory pages" });
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]);
});

test("P4/group_key: casefold groups, later display changes, untyped parents and More preserve literal order", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const base = indexFacts();
  const extra = (id: string, title: string) => ({ ...base.data[0]!, page_id: id, title });
  fixture.value.data = [
    { ...base.data[0]!, type: "Straße", type_key: "strasse" },
    { ...base.data[1]!, type: null, type_key: "strasse" },
  ];
  fixture.value.next_cursor = "same-group-next";
  fixture.answer = (request) =>
    new URL(request.url).searchParams.has("cursor")
      ? json({
          ...base,
          data: [
            { ...base.data[2]!, type: "STRASSE", type_key: "strasse" },
            { ...extra("0199a2b4-0000-7000-8000-000000000704", "Delta"), type: "é", type_key: "é" },
            { ...extra("0199a2b4-0000-7000-8000-000000000705", "Echo"), type: "e\u0301", type_key: "é" },
            {
              ...extra("0199a2b4-0000-7000-8000-000000000706", "Foxtrot"),
              type: "Added after sorting",
              type_key: null,
              parent_id: guide.id,
            },
            {
              ...extra("0199a2b4-0000-7000-8000-000000000707", "Golf"),
              type: null,
              type_key: null,
              parent_id: notes.id,
            },
            {
              ...extra("0199a2b4-0000-7000-8000-000000000708", "Hotel"),
              type: null,
              type_key: null,
              parent_id: guide.id,
            },
          ],
        })
      : json(fixture.value);
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("heading", { level: 2, name: "Straße" });
  await user.click(screen.getByRole("button", { name: "More" }));
  await screen.findByRole("link", { name: "Hotel" });
  expect(
    directory()
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent)
  ).toEqual(["Straße", "é", "Without a type · Guide.md", "Without a type · Notes.md"]);
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Hotel", "Golf"]);
  expect(directory().getByText("Added after sorting")).toBeTruthy();
  expect(fixture.requests.map((request) => new URL(request.url).search)).toEqual(["", "?cursor=same-group-next"]);
});

test.each([
  { sources: 0, label: "Guide/raw.md · 0 sources" },
  { sources: 2, label: "Guide/raw.md · 2 sources" },
])(
  "P4/details: summaries, tags, dates, caps and one current source-area link ($sources)",
  async ({ sources, label }) => {
    let lintReads = 0;
    const fixture = indexServer({
      [`GET /api/v0/notebooks/${notebookJSON.id}/lint`]: () => {
        lintReads += 1;
        throw new Error("Directory must not calculate health");
      },
    });
    fixture.server.notebook.mode = "llm_wiki";
    fixture.server.notebook.source_root_id = indexRaw.id;
    fixture.value.mode = "llm_wiki";
    fixture.value.source_area = { root_id: indexRaw.id, sources };
    renderApp(indexPath, fixture.server.app);
    await screen.findByRole("link", { name: label });
    expect(screen.getAllByRole("link", { name: label })).toHaveLength(1);
    expect(screen.getByRole("link", { name: label }).getAttribute("href")).toBe(
      `${indexHome}/health#undigested_sources`
    );
    expect(directory().getByText("First summary")).toBeTruthy();
    expect(directory().getByText("Summary shortened")).toBeTruthy();
    expect(directory().getByText("#plan/q4")).toBeTruthy();
    expect(directory().getByText("2 incoming links")).toBeTruthy();
    expect(directory().getByText("At least 1,000 incoming links")).toBeTruthy();
    expect(directory().getAllByText(/Oct [345], 2026/)).toHaveLength(3);
    expect(lintReads).toBe(0);
  }
);

test.each(["type", "tag"] as const)(
  "P4/filter_key/%s: exact URL/query and visible result change without moving focus",
  async (field) => {
    const user = userEvent.setup();
    const fixture = indexServer();
    fixture.answer = (request) => {
      const query = new URL(request.url).searchParams;
      return json({
        ...indexFacts(),
        data: query.has(field) ? [{ ...indexFacts().data[2]!, title: "Filtered Charlie" }] : indexFacts().data,
      });
    };
    const { router } = renderApp(indexPath, fixture.server.app);
    await screen.findByRole("option", { name: "note (1)" });
    const control = screen.getByRole(field === "type" ? "combobox" : "textbox", {
      name: field === "type" ? "Type" : "Tag",
    });
    if (field === "type") await user.selectOptions(control, "note");
    else {
      await user.type(control, "plan/q4");
      await user.keyboard("{Enter}");
    }
    await screen.findByRole("link", { name: "Filtered Charlie" });
    expect(titles()).toEqual(["Filtered Charlie"]);
    expect(new URLSearchParams(router.state.location.search).get(field)).toBe(field === "type" ? "note" : "plan/q4");
    expect(new URL(fixture.requests.at(-1)!.url).searchParams.get(field)).toBe(field === "type" ? "note" : "plan/q4");
    expect(document.activeElement).toBe(control);
  }
);

test("P4/catalogue_notice: exact colliding/long selections stay intact and bounded notices remain independent", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const selected = "é".repeat(512); // Exactly 1024 UTF-8 bytes, not 1024 characters.
  const another = "é".repeat(511) + "ax"; // Same display/key prefix, another exact 1024-byte value.
  fixture.value.types = [
    { type: "A", count: 2 },
    { type: "a", count: 3 },
    { type: another, count: 4 },
  ];
  fixture.value.types_complete = false;
  fixture.value.unfilterable_type_pages = 7;
  fixture.value.data[0]!.type = "é".repeat(128); // A long display prefix is not an eligible menu value.
  fixture.answer = (request) => {
    const type = new URL(request.url).searchParams.get("type");
    if (type === "A")
      return json({
        ...fixture.value,
        data: [
          { ...indexFacts().data[0]!, type: "A", type_key: "a" },
          { ...indexFacts().data[1]!, type: "A", type_key: "a" },
        ],
      });
    if (type === "a")
      return json({
        ...fixture.value,
        data: [
          { ...indexFacts().data[0]!, type: "a", type_key: "a" },
          { ...indexFacts().data[1]!, type: "a", type_key: "a" },
          { ...indexFacts().data[2]!, type: "a", type_key: "a" },
        ],
      });
    return json(fixture.value);
  };
  const { router } = renderApp(`${indexPath}?type=${encodeURIComponent(selected)}`, fixture.server.app);
  await screen.findByText("Only some filterable types are shown.");
  const choice = screen.getByRole("combobox", { name: "Type" }) as HTMLSelectElement;
  expect(choice.value).toBe(selected);
  expect(choice.selectedOptions[0]?.textContent).not.toMatch(/\(\d+\)/);
  expect(screen.getByText("Only some filterable types are shown.")).toBeTruthy();
  expect(screen.getByText("7 pages have type names too long to filter by.")).toBeTruthy();
  expect([...choice.options].map((option) => option.value)).toEqual(["", selected, "A", "a", another]);
  expect(new URL(fixture.requests[0]!.url).searchParams.get("type")).toBe(selected);
  expect(new URLSearchParams(router.state.location.search).get("type")).toBe(selected);
  expect(fixture.requests).toHaveLength(1);
  await user.selectOptions(choice, "A");
  await waitFor(() => expect(fixture.requests).toHaveLength(2));
  expect(new URL(fixture.requests[1]!.url).searchParams.get("type")).toBe("A");
  await waitFor(() => expect(titles()).toEqual(["Alpha", "Bravo"]));
  await user.selectOptions(choice, "a");
  await waitFor(() => expect(fixture.requests).toHaveLength(3));
  expect(new URL(fixture.requests[2]!.url).searchParams.get("type")).toBe("a");
  await waitFor(() => expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]));
});

test("P4/more_failure: a refused More stops being busy, says so, and reads again", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.value.data = [indexFacts().data[0]!];
  fixture.value.next_cursor = "next";
  let refused = true;
  fixture.answer = (request) => {
    if (!new URL(request.url).searchParams.has("cursor")) return json(fixture.value);
    return refused ? problem(404, "notebook.not_found") : json({ ...indexFacts(), data: [indexFacts().data[1]!] });
  };
  renderApp(indexPath, fixture.server.app);
  const more = await screen.findByRole("button", { name: "More" });
  await user.click(more);
  await screen.findByRole("alert");
  await waitFor(() => expect(more.getAttribute("aria-busy")).toBeNull());
  expect(titles()).toEqual(["Alpha"]);
  refused = false;
  await user.click(more);
  const added = await screen.findByRole("link", { name: "Bravo" });
  await waitFor(() => expect(document.activeElement).toBe(added));
});

test.each([false, true])("P4/focus: More adds a real page; reader moved=%s", async (moved) => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.value.data = [indexFacts().data[0]!];
  fixture.value.next_cursor = "next";
  const held = deferred<Response>();
  fixture.answer = (request) => (new URL(request.url).searchParams.has("cursor") ? held.promise : json(fixture.value));
  renderApp(indexPath, fixture.server.app);
  const more = await screen.findByRole("button", { name: "More" });
  await user.click(more);
  await waitFor(() => expect(fixture.requests).toHaveLength(2));
  const tag = screen.getByRole("textbox", { name: "Tag" });
  if (moved) {
    await user.click(tag);
    fireEvent.wheel(window);
  }
  await act(async () => held.resolve(json({ ...indexFacts(), data: indexFacts().data.slice(1) })));
  const added = await screen.findByRole("link", { name: "Bravo" });
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]);
  await waitFor(() => expect(document.activeElement).toBe(moved ? tag : added));
});

test.each([
  { complete: false, long: 0, partial: true, tooLong: false },
  { complete: true, long: 3, partial: false, tooLong: true },
])("P4/catalogue_notice independent flags: $complete/$long", async ({ complete, long, partial, tooLong }) => {
  const fixture = indexServer();
  fixture.value.types_complete = complete;
  fixture.value.unfilterable_type_pages = long;
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  await waitFor(() => {
    expect(screen.queryByText("Only some filterable types are shown.") !== null).toBe(partial);
    expect(screen.queryByText("3 pages have type names too long to filter by.") !== null).toBe(tooLong);
  });
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]);
});

test("P4/open_row: a row opens its page arrived at, the page's title taking the focus", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.value.data = [{ ...indexFacts().data[0]!, page_id: guide.id, title: guide.name }];
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  await user.click(directory().getByRole("link", { name: guide.name }));
  const title = await screen.findByRole("heading", { level: 1, name: guide.name });
  await waitFor(() => expect(document.activeElement).toBe(title));
});

test.each([false, true])(
  "P4/reopen: the link that goes with the list gives the focus to the title; arrived=%s",
  async (entered) => {
    const user = userEvent.setup();
    const fixture = indexServer();
    fixture.value.mode = "llm_wiki";
    const { router } = renderApp(indexHome, fixture.server.app);
    await screen.findByRole("heading", { level: 1, name: "Plans" });
    await act(() => router.navigate(`${indexPath}?tag=plan`, { state: entered ? arrived : undefined }));
    const reopen = await screen.findByRole("link", { name: "Reopen directory" });
    fixture.value.mode = "standard";
    const reads = fixture.requests.length;
    reopen.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(screen.queryByRole("link", { name: "Reopen directory" })).toBeNull());
    expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Directory" }));
    expect(fixture.requests).toHaveLength(reads + 1);
    expect(new URL(fixture.requests.at(-1)!.url).search).toBe("?tag=plan");
    expect(router.state.location.pathname + router.state.location.search).toBe(`${indexPath}?tag=plan`);
  }
);

test("P4/clear_focus: Clear filters keeps the focus as nothing is left to clear, and then does nothing", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  renderApp(`${indexPath}?tag=plan`, fixture.server.app);
  const clear = await screen.findByRole("button", { name: "Clear filters" });
  await screen.findByRole("region", { name: "Directory pages" });
  expect(clear.getAttribute("aria-disabled")).toBeNull();
  clear.focus();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(new URL(fixture.requests.at(-1)!.url).search).toBe(""));
  expect(clear.getAttribute("aria-disabled")).toBe("true");
  expect(document.activeElement).toBe(clear);
  const reads = fixture.requests.length;
  await user.keyboard("{Enter}");
  expect(fixture.requests).toHaveLength(reads);
});

test("P4/catalogue_stays: every type stays choosable while the next filter is read, a failure included", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("option", { name: "Topic (1)" });
  const held = deferred<Response>();
  fixture.answer = () => held.promise;
  const type = screen.getByRole("combobox", { name: "Type" });
  const options = () =>
    within(type)
      .getAllByRole("option")
      .map((option) => option.textContent);
  const every = ["All types", "Note (1)", "note (1)", "Topic (1)"];
  expect(options()).toEqual(every);
  await user.selectOptions(type, "Note");
  await waitFor(() => expect(fixture.requests).toHaveLength(2));
  expect(options()).toEqual(every);
  await act(async () => held.resolve(problem(404, "notebook.not_found")));
  await screen.findByRole("alert");
  expect(options()).toEqual(every);
});

test.each([
  { query: "", said: "This notebook has no pages yet." },
  { query: "?tag=none", said: "No pages match these filters." },
])("P4/empty: an empty list says why, filtered '$query'", async ({ query, said }) => {
  const fixture = indexServer();
  fixture.value.data = [];
  renderApp(`${indexPath}${query}`, fixture.server.app);
  await screen.findByText(said);
});

test("P4/blank_type: a type of blanks cut short leaves no empty line under the title", async () => {
  const fixture = indexServer();
  const blank = " ".repeat(256);
  fixture.value.data = [{ ...indexFacts().data[0]!, type: blank, type_key: " ".repeat(64) }];
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  const row = directory().getByRole("link", { name: "Alpha" }).closest("li")!;
  expect([...row.querySelectorAll("p")].filter((line) => line.textContent === blank)).toEqual([]);
});

test("P4/parent_pending: an untyped group says nothing of its parent until the tree is read", async () => {
  const held = deferred<Response>();
  const fixture = indexServer({ [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: () => held.promise });
  fixture.value.data = [{ ...indexFacts().data[1]!, type: null, type_key: null }];
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("heading", { level: 2, name: "Without a type · …" });
  expect(screen.queryByText(/Parent unavailable/)).toBeNull();
});

test("P4/crossed_mode: a later page of another mode joins nothing and leaves the way out", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  fixture.value.data = [indexFacts().data[0]!];
  fixture.value.next_cursor = "next";
  fixture.answer = (request) =>
    new URL(request.url).searchParams.has("cursor")
      ? json({ ...indexFacts(), mode: "llm_wiki", data: [indexFacts().data[1]!] })
      : json(fixture.value);
  renderApp(indexPath, fixture.server.app);
  const more = await screen.findByRole("button", { name: "More" });
  more.focus();
  await user.keyboard("{Enter}");
  await screen.findByRole("link", { name: "Reopen directory" });
  expect(titles()).toEqual(["Alpha"]);
  expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  await waitFor(() =>
    expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Directory" }))
  );
});

test("P4/filters_together: choosing a type keeps the applied tag", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const { router } = renderApp(`${indexPath}?tag=plan`, fixture.server.app);
  await screen.findByRole("option", { name: "note (1)" });
  await user.selectOptions(screen.getByRole("combobox", { name: "Type" }), "note");
  await waitFor(() => expect(router.state.location.search).toBe("?type=note&tag=plan"));
  await waitFor(() => expect(new URL(fixture.requests.at(-1)!.url).search).toBe("?type=note&tag=plan"));
});

test("P4/tag_follows: the Tag box follows the address, and Clear also empties text not applied", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const { router } = renderApp(`${indexPath}?tag=plan`, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  const tag = screen.getByRole("textbox", { name: "Tag" }) as HTMLInputElement;
  expect(tag.value).toBe("plan");
  await act(() => router.navigate(indexPath));
  await waitFor(() => expect(tag.value).toBe(""));
  await screen.findByRole("region", { name: "Directory pages" });
  const reads = fixture.requests.length;
  await user.type(tag, "typed");
  await user.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(tag.value).toBe("");
  expect(router.state.location.search).toBe("");
  expect(fixture.requests).toHaveLength(reads);
});

test("P4/long_label: a long type is shortened on its label only", async () => {
  const user = userEvent.setup();
  const fixture = indexServer();
  const long = "t".repeat(100);
  fixture.value.types = [{ type: long, count: 2 }];
  const { router } = renderApp(indexPath, fixture.server.app);
  const option = await screen.findByRole("option", { name: `${"t".repeat(79)}… (2)` });
  expect((option as HTMLOptionElement).value).toBe(long);
  await user.selectOptions(screen.getByRole("combobox", { name: "Type" }), option);
  await waitFor(() => expect(new URLSearchParams(router.state.location.search).get("type")).toBe(long));
});

test("P4/tree_failure: a tree that cannot be read says so above the rows, which stay", async () => {
  const fixture = indexServer();
  fixture.server.nodesDown = true;
  renderApp(indexPath, fixture.server.app);
  await screen.findByRole("region", { name: "Directory pages" });
  const main = within(screen.getByRole("main"));
  await waitFor(() => expect(main.getByRole("button", { name: "Try again" })).toBeTruthy());
  expect(titles()).toEqual(["Alpha", "Bravo", "Charlie"]);
});
