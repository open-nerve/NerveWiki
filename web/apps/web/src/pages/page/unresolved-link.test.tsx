import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import { appLinks } from "../../reading/app-links";
import { unresolvedLinks } from "../../reading/unresolved-links";
import { json, notebookJSON, problem } from "../../test/fakes";
import { guide, install, notes, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// A link to a page that is not there, acted on (M6/P6 design 7).

afterEach(() => vi.useRealTimers());

const enhancements = [appLinks, unresolvedLinks];

/** linkTo is the HTML of a link to a page not there, its target target, showing text. */
const linkTo = (target: string, text = target, kind = "nw-wikilink nw-unresolved") =>
  `<a class="${kind}" data-nw-target="${target}">${text}</a>`;

/** open shows Guide, whose view is html, to role, and answers its server. */
async function open(html: string, role: "admin" | "editor" | "reader" = "editor", answers = {}) {
  const server = pageServer({ role, answers });
  server.views.set(guide.id, { html: `<p>${html}</p>`, revision: 1 });
  const view = renderApp(pagePath(guide.id), server.app, { enhancements });
  await screen.findByRole("article", { name: "Guide" });
  return { ...view, server };
}

/** button is the link named name, a button now. */
const button = (name: string) => screen.findByRole("button", { name });

test("a writer's link asks where its page would go, and creates it there once confirmed: the tab goes to it", async () => {
  const { server, router } = await open(linkTo("Install/x", "x"));
  server.landings.set("Install/x", { node_id: null, landing: { parent_id: install.id, title: "x" }, reason: null });

  await userEvent.click(await button("x"));
  const dialog = await screen.findByRole("alertdialog", { name: "Create page “x”?" });
  expect(within(dialog).getByText("It goes under “Guide / Install”.")).toBeTruthy();
  await userEvent.click(within(dialog).getByRole("button", { name: "Create page" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "x" });
  const made = server.nodes.find((node) => node.name === "x");
  expect(router.state.location.pathname).toBe(pagePath(made?.id ?? ""));
  await waitFor(() => expect(document.activeElement).toBe(heading));
  const sent = server.sent.filter((each) => !each.startsWith("GET nodes") && !each.startsWith("GET view x"));
  expect(sent.slice(sent.indexOf("GET landing Install/x"))).toEqual([
    "GET landing Install/x",
    "POST x under Install",
    // The view the link was in, read again for when the reader comes back.
    "GET view Guide",
  ]);
});

test("a landing at the top level says so; cancelled, the dialog gives the focus back to the link", async () => {
  const { server } = await open(linkTo("Top"));

  await userEvent.click(await button("Top"));
  const dialog = await screen.findByRole("alertdialog", { name: "Create page “Top”?" });
  expect(within(dialog).getByText("It goes at the notebook's top level.")).toBeTruthy();
  await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Top" })));
  expect(server.sent.filter((each) => each.startsWith("POST"))).toEqual([]);
});

test("a link that leads to a page by now goes there, without a question", async () => {
  const { server, router } = await open(linkTo("Notes"));
  server.landings.set("Notes", { node_id: notes.id, landing: null, reason: null });

  await userEvent.click(await button("Notes"));

  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

test("a link with no landing says why; OK gives the focus back to the link", async () => {
  const reasons = {
    title_invalid: "Its name cannot be a page's title.",
    parent_missing: "A page on its path does not exist.",
    too_deep: "Its page would nest more than 10 levels deep.",
    not_resolvable:
      "A page created for it would not be the one it leads to: another page answers to its name, or the name is written otherwise.",
    target_invalid: "This link's target cannot name a page.",
  } as const;
  const { server } = await open(
    [...Object.keys(reasons), "long"].map((target) => linkTo(target)).join(" "),
    "editor",
    {}
  );
  for (const reason of Object.keys(reasons) as (keyof typeof reasons)[]) {
    server.landings.set(reason, { node_id: null, landing: null, reason });
  }
  // A target longer than the server takes is refused as such.
  server.landings.set("long", () =>
    problem(422, "validation_failed", { errors: [{ field: "target", code: "too_long", message: "long" }] })
  );

  const cases: [string, string][] = [...Object.entries(reasons), ["long", reasons.target_invalid]];
  for (const [target, text] of cases) {
    // oxlint-disable-next-line no-await-in-loop -- one dialog after another
    await userEvent.click(await button(target));
    // oxlint-disable-next-line no-await-in-loop -- one dialog after another
    const dialog = await screen.findByRole("alertdialog", { name: `“${target}” does not exist` });
    expect(within(dialog).getByText(text), target).toBeTruthy();
    // oxlint-disable-next-line no-await-in-loop -- one dialog after another
    await userEvent.click(within(dialog).getByRole("button", { name: "OK" }));
    // oxlint-disable-next-line no-await-in-loop -- one dialog after another
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: target })));
  }
});

test("to a reader, a link says its page is not there; to anyone, an embed and an image do: no question goes out", async () => {
  const html = `${linkTo("x")} ${linkTo("E", "E", "nw-wikilink nw-embed nw-unresolved")} <span class="nw-image">pic ${linkTo("p.png", "p.png", "nw-unresolved")}</span>`;
  const texts = {
    x: "No page has this name. The notebook's editors can create it from this link.",
    E: "No page has this name. An embed shows a page; it does not create one.",
    "p.png": "No page has this name. An image shows what is there; it does not create a page.",
  };
  for (const role of ["reader", "admin"] as const) {
    // oxlint-disable-next-line no-await-in-loop -- one role after another
    const { server, unmount } = await open(html, role);
    for (const [target, text] of Object.entries(texts)) {
      if (role === "admin" && target === "x") {
        continue;
      }
      // oxlint-disable-next-line no-await-in-loop -- one dialog after another
      await userEvent.click(await button(target));
      // oxlint-disable-next-line no-await-in-loop -- one dialog after another
      const dialog = await screen.findByRole("alertdialog", { name: `“${target}” does not exist` });
      expect(within(dialog).getByText(text), `${role} ${target}`).toBeTruthy();
      // oxlint-disable-next-line no-await-in-loop -- one dialog after another
      await userEvent.click(within(dialog).getByRole("button", { name: "OK" }));
    }
    expect(server.sent.filter((each) => each.startsWith("GET landing"))).toEqual([]);
    unmount();
  }
});

test("a title taken since goes to the page the link leads to by then; else the dialog says why", async () => {
  let asked = 0;
  const { server, router } = await open(`${linkTo("Notes", "Notes again")} ${linkTo("Guide", "Guide again")}`);
  // Each a landing first, as though the page were not there yet; Notes leads to its page when asked again.
  server.landings.set("Notes", () => {
    asked += 1;
    return json(
      asked === 1
        ? { node_id: null, landing: { parent_id: null, title: "Notes" }, reason: null }
        : { node_id: notes.id, landing: null, reason: null }
    );
  });
  server.landings.set("Guide", { node_id: null, landing: { parent_id: null, title: "Guide" }, reason: null });

  await userEvent.click(await button("Guide again"));
  const dialog = await screen.findByRole("alertdialog", { name: "Create page “Guide”?" });
  await userEvent.click(within(dialog).getByRole("button", { name: "Create page" }));
  expect(
    await within(dialog).findByText(
      "A page under the same parent already has this title (titles differ in more than case)."
    )
  ).toBeTruthy();
  await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await userEvent.click(await button("Notes again"));
  await userEvent.click(
    within(await screen.findByRole("alertdialog", { name: "Create page “Notes”?" })).getByRole("button", {
      name: "Create page",
    })
  );
  expect(await screen.findByRole("heading", { level: 1, name: "Notes" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(pagePath(notes.id));
});

test("a parent deleted since the landing was read says so in the dialog", async () => {
  const { server } = await open(linkTo("Install/x", "x"), "editor", {
    [`POST /api/v0/notebooks/${notebookJSON.id}/pages`]: () =>
      problem(422, "validation_failed", { errors: [{ field: "parent_id", code: "not_found", message: "gone" }] }),
  });
  server.landings.set("Install/x", { node_id: null, landing: { parent_id: install.id, title: "x" }, reason: null });

  await userEvent.click(await button("x"));
  const dialog = await screen.findByRole("alertdialog", { name: "Create page “x”?" });
  await userEvent.click(within(dialog).getByRole("button", { name: "Create page" }));

  expect(await within(dialog).findByText("The page it would go under no longer exists.")).toBeTruthy();
});

test("a landing refused otherwise is the page's to say, as a refusal of Edit is, until the next question", async () => {
  const { server } = await open(`${linkTo("x")} ${linkTo("y")}`);
  server.landings.set("x", () => problem(403, "forbidden"));

  await userEvent.click(await button("x"));

  expect(await screen.findByText("You do not have permission to do this.")).toBeTruthy();
  expect(screen.queryByRole("alertdialog")).toBeNull();
  await userEvent.click(await button("y"));
  expect(await screen.findByRole("alertdialog", { name: "Create page “y”?" })).toBeTruthy();
  expect(screen.queryByText("You do not have permission to do this.")).toBeNull();
});

test("a target past a proxy's limit on the address (414) has no landing", async () => {
  const { server } = await open(linkTo("long"));
  server.landings.set("long", () => new Response("URI Too Long", { status: 414 }));

  await userEvent.click(await button("long"));

  const dialog = await screen.findByRole("alertdialog", { name: "“long” does not exist" });
  expect(within(dialog).getByText("This link's target cannot name a page.")).toBeTruthy();
});

test("a link that leads to a page another tab made, which this tab's tree does not have yet, reads the tree and goes there", async () => {
  const { server, router } = await open(linkTo("Fresh"));
  const fresh = { ...notes, id: "0199a2b4-0000-7000-8000-0000000000f8", name: "Fresh" };
  server.nodes = [...server.nodes, fresh];
  server.landings.set("Fresh", { node_id: fresh.id, landing: null, reason: null });

  await userEvent.click(await button("Fresh"));

  expect(await screen.findByRole("heading", { level: 1, name: "Fresh" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(pagePath(fresh.id));
  expect(screen.queryByRole("heading", { name: "Page not found" })).toBeNull();
});

test("a landing answered once the reader left the page goes nowhere", async () => {
  let answer!: () => void;
  const { server, router } = await open(linkTo("Notes"));
  server.landings.set(
    "Notes",
    () =>
      new Promise<Response>((resolve) => {
        answer = () => resolve(new Response(JSON.stringify({ node_id: notes.id, landing: null, reason: null })));
      })
  );

  await userEvent.click(await button("Notes"));
  await waitFor(() => expect(server.sent).toContain("GET landing Notes"));
  const home = `/lab/notebooks/${notebookJSON.id}`;
  await act(() => router.navigate(home));
  act(() => answer());
  await new Promise((resolve) => setTimeout(resolve, 50));

  expect(router.state.location.pathname).toBe(home);
});

test("one link at a time: busy while the server answers, a second activation asks nothing", async () => {
  let answer!: () => void;
  const { server } = await open(`${linkTo("x")} ${linkTo("y")}`);
  server.landings.set(
    "x",
    () =>
      new Promise<Response>((resolve) => {
        answer = () => resolve(new Response(JSON.stringify({ node_id: null, landing: null, reason: "too_deep" })));
      })
  );

  const x = await button("x");
  await userEvent.click(x);
  await waitFor(() => expect(x.getAttribute("aria-busy")).toBe("true"));
  await userEvent.click(x);
  await userEvent.click(await button("y"));
  expect(server.sent.filter((each) => each.startsWith("GET landing"))).toEqual(["GET landing x"]);

  act(() => answer());
  expect(await screen.findByRole("alertdialog", { name: "“x” does not exist" })).toBeTruthy();
  expect(x.hasAttribute("aria-busy")).toBe(false);
});

test("Enter opens the dialog; with the view read again meanwhile, the focus goes back to the link at the same place among those to its target, else to the article", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const { server } = await open(`${linkTo("T", "first")} ${linkTo("T", "second")}`);
  const readAgain = async (html: string, revision: number) => {
    server.views.set(guide.id, { html, revision });
    await act(() => vi.advanceTimersByTimeAsync(6_000));
    act(() => void window.dispatchEvent(new Event("focus")));
    const text = Object.assign(document.createElement("template"), { innerHTML: html }).content.textContent;
    // The dialog open, the rest of the page is hidden from the accessibility tree.
    await waitFor(() => expect(screen.getByRole("article", { name: "Guide", hidden: true }).textContent).toBe(text));
  };

  (await button("second")).focus();
  await userEvent.keyboard("{Enter}");
  let dialog = await screen.findByRole("alertdialog", { name: "Create page “T”?" });
  await readAgain(`<p>${linkTo("T", "one")} ${linkTo("T", "two")}</p>`, 2);
  await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "two" })));

  await userEvent.keyboard("{Enter}");
  dialog = await screen.findByRole("alertdialog", { name: "Create page “T”?" });
  await readAgain("<p>T is gone</p>", 3);
  await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("article", { name: "Guide" })));
});
