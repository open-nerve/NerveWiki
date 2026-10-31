import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { json, notebookJSON, problem } from "../../test/fakes";
import { deferred, healthPath, healthServer, wantedFacts } from "../../test/health-server";
import { guide } from "../../test/page-server";
import { renderApp } from "../../test/render";

afterEach(() => vi.useRealTimers());

async function opened(fixture = healthServer()) {
  const view = renderApp(healthPath, fixture.server.app);
  await screen.findByRole("heading", { name: "Wanted pages" });
  return { ...fixture, ...view };
}

function wanted() {
  const section = screen.getByRole("heading", { name: "Wanted pages" }).closest("section");
  if (section === null) throw new Error("missing wanted card");
  return within(section);
}

test("More forwards the preview cursor and focuses the first actual addition even on a nonfinal page", async () => {
  const user = userEvent.setup();
  const fixture = await opened();
  await user.click(wanted().getByRole("button", { name: "More" }));
  await waitFor(() => expect(wanted().getAllByRole("listitem")).toHaveLength(55));
  expect(new URL(fixture.pages[0]!.url).searchParams.get("cursor")).toBe(fixture.value.rules.wanted?.[4]?.cursor);
  expect(wanted().getByRole("button", { name: "More" })).toBeTruthy();
  expect(document.activeElement).toBe(screen.getByText("Missing 006").closest("li"));
  expect(wanted().getByRole("status").textContent).toContain("50");
  expect(fixture.summaries).toHaveLength(1);
});

test.each(["wheel", "keydown", "pointerdown"] as const)(
  "More does not steal focus after the reader's %s action",
  async (action) => {
    const user = userEvent.setup();
    const fixture = healthServer();
    const answer = deferred<Response>();
    fixture.responses.page = () => answer.promise;
    await opened(fixture);
    const button = wanted().getByRole("button", { name: "More" });
    await user.click(button);
    if (action === "wheel") fireEvent.wheel(window);
    else if (action === "keydown") fireEvent.keyDown(button, { key: "ArrowDown" });
    else fireEvent.pointerDown(window, { button: 0 });
    await act(async () => answer.resolve(fixture.pageOf(wantedFacts(6).slice(5), wantedFacts(6)[5]!.cursor)));
    await screen.findByText("Missing 006");
    expect(document.activeElement).toBe(button);
  }
);

test("rapid double clicking More sends one page, and an empty tail returns focus to the last actual item", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const answer = deferred<Response>();
  fixture.responses.page = () => answer.promise;
  await opened(fixture);
  await user.dblClick(wanted().getByRole("button", { name: "More" }));
  expect(fixture.pages).toHaveLength(1);
  await act(async () => answer.resolve(fixture.pageOf([])));
  await waitFor(() => expect(wanted().queryByRole("button", { name: "More" })).toBeNull());
  expect(wanted().getAllByRole("listitem")).toHaveLength(5);
  expect(document.activeElement).toBe(screen.getByText("Missing 005").closest("li"));
});

test("reader activity also prevents the disappearing final-page button from moving focus", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const answer = deferred<Response>();
  fixture.responses.page = () => answer.promise;
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  fireEvent.wheel(window);
  await act(async () => answer.resolve(fixture.pageOf([])));
  await waitFor(() => expect(wanted().queryByRole("button", { name: "More" })).toBeNull());
  expect(document.activeElement).not.toBe(screen.getByRole("heading", { name: "Wanted pages" }));
  expect(document.activeElement).not.toBe(screen.getByText("Missing 005").closest("li"));
});

test("duplicate pages add no row or focus target; a later unique item and repeated announcements still work", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const data = wantedFacts(7);
  let reads = 0;
  fixture.responses.page = () => {
    reads += 1;
    return reads < 3 ? fixture.pageOf(data.slice(0, 2), data[4]!.cursor) : fixture.pageOf(data.slice(1, 7));
  };
  await opened(fixture);
  const output = wanted().getByRole("status");
  expect(output.textContent).toBe("");
  const more = wanted().getByRole("button", { name: "More" });
  await user.click(more);
  await waitFor(() => expect(output.textContent).not.toBe(""));
  const first = output.textContent;
  expect(wanted().getAllByRole("listitem")).toHaveLength(5);
  expect(document.activeElement).toBe(more);
  await user.click(more);
  await waitFor(() => expect(output.textContent).not.toBe(first));
  expect(output.textContent?.trim()).toBe(first?.trim());
  await user.click(more);
  await screen.findByText("Missing 007");
  expect(wanted().getAllByRole("listitem")).toHaveLength(7);
  expect(document.activeElement).toBe(screen.getByText("Missing 006").closest("li"));
  expect(wanted().getByRole("status")).toBe(output);
});

test("card busy with retained data is manual even beyond every retry delay, and retry reads only that cursor", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const fixture = healthServer();
  let busy = true;
  fixture.responses.page = (request) =>
    busy ? problem(503, "server_busy", {}, { "Retry-After": "1" }) : fixture.page(request);
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  expect(await wanted().findByRole("alert")).toBeTruthy();
  expect(wanted().getAllByRole("listitem")).toHaveLength(5);
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.pages).toHaveLength(1);
  expect(fixture.summaries).toHaveLength(1);
  busy = false;
  await user.click(wanted().getByRole("button", { name: "Try again" }));
  await screen.findByText("Missing 006");
  expect(fixture.pages).toHaveLength(2);
  expect(new URL(fixture.pages[1]!.url).searchParams.get("cursor")).toBe(
    new URL(fixture.pages[0]!.url).searchParams.get("cursor")
  );
  expect(fixture.summaries).toHaveLength(1);
  expect(wanted().queryByRole("alert")).toBeNull();
});

test("a card network failure retains results while its existing finite retry policy still runs", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const fixture = healthServer();
  fixture.responses.page = () => {
    throw new TypeError("offline");
  };
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  await wanted().findByRole("alert");
  await act(() => vi.advanceTimersByTimeAsync(300_000));
  expect(fixture.pages).toHaveLength(6);
  expect(wanted().getAllByRole("listitem")).toHaveLength(5);
  expect(fixture.summaries).toHaveLength(1);
});

test("a failed operation's timer cannot retry a newer operation after manual success and another page", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
  const fixture = healthServer();
  let reads = 0;
  fixture.responses.page = () => {
    reads += 1;
    if (reads === 1) throw new TypeError("offline");
    if (reads < 4) return fixture.pageOf(wantedFacts(6).slice(5), wantedFacts(6)[5]!.cursor);
    return fixture.pageOf(wantedFacts(7).slice(6));
  };
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  await wanted().findByRole("alert");
  await user.click(wanted().getByRole("button", { name: "Try again" }));
  await screen.findByText("Missing 006");
  await user.click(wanted().getByRole("button", { name: "More" }));
  await waitFor(() => expect(wanted().getByRole("status").textContent).toContain("0 more issues shown."));
  const said = wanted().getByRole("status").textContent;
  const at = document.activeElement;
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  expect(fixture.pages).toHaveLength(3);
  expect(wanted().getAllByRole("listitem")).toHaveLength(6);
  expect(screen.queryByText("Missing 007")).toBeNull();
  expect(wanted().getByRole("status").textContent).toBe(said);
  expect(document.activeElement).toBe(at);
});

test.each([false, true])(
  "an old same-cursor More cannot append, announce, error or focus after a successful summary reset (late error=%s)",
  async (failed) => {
    const user = userEvent.setup();
    const fixture = healthServer();
    const answer = deferred<Response>();
    fixture.responses.page = () => answer.promise;
    await opened(fixture);
    await user.click(wanted().getByRole("button", { name: "More" }));
    fixture.value.computed_at = "2026-11-01T17:25:00Z";
    await user.click(screen.getByRole("button", { name: "Recheck" }));
    await screen.findByText(/Nov 1, 2026/);
    const before = wanted().getByRole("status").textContent;
    const active = document.activeElement;
    await act(async () =>
      answer.resolve(
        failed ? problem(503, "server_busy", {}, { "Retry-After": "1" }) : fixture.pageOf(wantedFacts(6).slice(5))
      )
    );
    expect(wanted().getAllByRole("listitem")).toHaveLength(5);
    expect(screen.queryByText("Missing 006")).toBeNull();
    expect(wanted().getByRole("status").textContent).toBe(before);
    expect(document.activeElement).toBe(active);
    expect(wanted().queryByRole("alert")).toBeNull();
  }
);

test.each(["More", "Try again"])(
  "a failed refresh retains the expanded card and permits fresh-generation %s at the same cursor",
  async (action) => {
    const user = userEvent.setup();
    const fixture = healthServer();
    const old = deferred<Response>();
    let calls = 0;
    fixture.responses.page = () => {
      calls += 1;
      if (calls === 1) return fixture.pageOf(wantedFacts(6).slice(5), wantedFacts()[5]!.cursor);
      if (calls === 2 && action === "Try again") return problem(503, "server_busy", {}, { "Retry-After": "1" });
      if (calls === 2) return old.promise;
      return fixture.pageOf(wantedFacts(7).slice(6));
    };
    await opened(fixture);
    await user.click(wanted().getByRole("button", { name: "More" }));
    await screen.findByText("Missing 006");
    await user.click(wanted().getByRole("button", { name: "More" }));
    if (action === "Try again") await wanted().findByRole("alert");
    fixture.responses.summary = () => problem(503, "server_busy", {}, { "Retry-After": "1" });
    await user.click(screen.getByRole("button", { name: "Recheck" }));
    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThanOrEqual(1));
    expect(screen.getByText(/Oct 31, 2026/)).toBeTruthy();
    expect(wanted().getAllByRole("listitem")).toHaveLength(6);
    await user.click(wanted().getByRole("button", { name: action }));
    await screen.findByText("Missing 007");
    expect(fixture.pages).toHaveLength(3);
    expect(new URL(fixture.pages[2]!.url).searchParams.get("cursor")).toBe(
      new URL(fixture.pages[1]!.url).searchParams.get("cursor")
    );
    await act(async () => old.resolve(fixture.pageOf([{ ...wantedFacts(8)[7]!, target: "Late old item" }])));
    expect(screen.queryByText("Late old item")).toBeNull();
    expect(wanted().getAllByRole("listitem")).toHaveLength(7);
    expect(fixture.summaries).toHaveLength(2);
  }
);

test("a successful refresh removes old expanded items and retained card errors together", async () => {
  const user = userEvent.setup();
  const fixture = await opened();
  await user.click(wanted().getByRole("button", { name: "More" }));
  await screen.findByText("Missing 055");
  const missing = within(screen.getByRole("heading", { name: "Missing summary" }).closest("section")!);
  await user.click(missing.getByRole("button", { name: "More" }));
  await waitFor(() => expect(missing.getAllByRole("listitem")).toHaveLength(6));
  fixture.responses.page = () => problem(503, "server_busy", {}, { "Retry-After": "1" });
  await user.click(wanted().getByRole("button", { name: "More" }));
  await wanted().findByRole("alert");
  fixture.value.rules.wanted = wantedFacts(2);
  fixture.value.computed_at = "2026-11-01T17:25:00Z";
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await screen.findByText(/Nov 1, 2026/);
  expect(wanted().getAllByRole("listitem")).toHaveLength(2);
  expect(wanted().queryByRole("alert")).toBeNull();
  expect(wanted().queryByRole("button", { name: "More" })).toBeNull();
  expect(screen.queryByText("Missing 055")).toBeNull();
  const renewed = within(screen.getByRole("heading", { name: "Missing summary" }).closest("section")!);
  expect(renewed.getAllByRole("listitem")).toHaveLength(5);
  expect(renewed.getByRole("button", { name: "More" })).toBeTruthy();
});

test("a page mode mismatch locks every More until a fresh matching summary", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.responses.page = () => fixture.pageOf(wantedFacts(6).slice(5), null, "standard");
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  await screen.findByText("The notebook mode has changed. Recheck its health.");
  expect(wanted().getAllByRole("listitem")).toHaveLength(5);
  await user.click(wanted().getByRole("button", { name: "More" }));
  expect(fixture.pages).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Recheck" }));
  await waitFor(() => expect(screen.queryByText("The notebook mode has changed. Recheck its health.")).toBeNull());
  expect(wanted().getByRole("button", { name: "More" }).getAttribute("aria-disabled")).not.toBe("true");
});

test("a dedicated rule's 409 retains its preview and locks every More", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  fixture.responses.page = () => problem(409, "notebook.not_llm_wiki");
  await opened(fixture);
  const section = screen.getByRole("heading", { name: "Missing summary" }).closest("section")!;
  await user.click(within(section).getByRole("button", { name: "More" }));
  await screen.findByText("The notebook mode has changed. Recheck its health.");
  expect(within(section).getAllByRole("listitem")).toHaveLength(5);
  await user.click(wanted().getByRole("button", { name: "More" }));
  expect(fixture.pages).toHaveLength(1);
});

test("complete item keys keep same-prefix wanted groups and same-page link/nonlink findings across pages", async () => {
  const user = userEvent.setup();
  const fixture = healthServer();
  const prefix = "界".repeat(85) + "x";
  fixture.value.rules.wanted = wantedFacts(7, prefix);
  fixture.value.rules.invalid_sources = Array.from({ length: 6 }, (_, index) => {
    const hash = index.toString(16).padStart(64, "0");
    return {
      rule: "invalid_sources",
      item_key: `s:${guide.id}:${hash}`,
      page_id: guide.id,
      target: index === 0 || index === 5 ? "Broken" : `Other ${index}`,
      target_cut: false,
      not_link: index === 5,
      cursor: btoa(
        JSON.stringify({ v: 1, p: { v: 1, r: "invalid_sources", n: notebookJSON.id, s: "I", h: hash, i: guide.id } })
      )
        .replaceAll("+", "-")
        .replaceAll("/", "_")
        .replace(/=+$/, ""),
    };
  });
  await opened(fixture);
  await user.click(wanted().getByRole("button", { name: "More" }));
  await waitFor(() => expect(wanted().getAllByText(prefix)).toHaveLength(7));
  expect(wanted().getAllByRole("listitem")).toHaveLength(7);
  const invalid = within(screen.getByRole("heading", { name: "Invalid sources" }).closest("section")!);
  await user.click(invalid.getByRole("button", { name: "More" }));
  await waitFor(() => expect(invalid.getAllByText("Broken")).toHaveLength(2));
  expect(invalid.getAllByRole("listitem")).toHaveLength(6);
  expect(invalid.getAllByText("This value is not a single link.")).toHaveLength(1);
});

test.each([false, true])(
  "an opaque StartCursor is forwarded unchanged; an initially empty list handles empty tail=%s",
  async (empty) => {
    const user = userEvent.setup();
    const fixture = healthServer();
    const start = btoa(JSON.stringify({ v: 1, p: { v: 1, r: "wanted", n: notebookJSON.id, s: "I", b: true } }))
      .replaceAll("+", "-")
      .replaceAll("/", "_")
      .replace(/=+$/, "");
    fixture.responses.summary = async (request) => {
      const response = await fixture.summary(request);
      const body = await response.json();
      body.rules[0].items = [];
      body.rules[0].next_cursor = start;
      return json(body);
    };
    if (empty) fixture.responses.page = () => fixture.pageOf([]);
    await opened(fixture);
    await user.click(wanted().getByRole("button", { name: "More" }));
    await waitFor(() => expect(fixture.pages).toHaveLength(1));
    expect(new URL(fixture.pages[0]!.url).searchParams.get("cursor")).toBe(start);
    if (empty) {
      await waitFor(() => expect(wanted().queryByRole("button", { name: "More" })).toBeNull());
      expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Wanted pages" }));
    } else {
      await screen.findByText("Missing 001");
      expect(wanted().getAllByRole("listitem")).toHaveLength(50);
    }
  }
);
