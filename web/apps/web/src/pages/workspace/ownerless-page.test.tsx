import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";

import type { Notebook } from "../../services/notebook.service";
import type { NotebookAuditEvent, NotebookAuditEventPage, OwnerlessNotebook } from "../../services/ownerless.service";
import type { Workspace } from "../../services/workspace.service";
import {
  auditEventJSON,
  json,
  notebookJSON,
  ownerlessJSON,
  problem,
  signedInApp,
  workspaceJSON,
  type Answer,
} from "../../test/fakes";
import { renderApp } from "../../test/render";

// A workspace's ownerless notebooks and their audit (M3/P5 design 3.3).

const page = "/lab/settings/ownerless";
const atlas: OwnerlessNotebook = {
  ...ownerlessJSON,
  id: "0199a2b4-0000-7000-8000-0000000000e2",
  name: "Atlas",
  workspace_access: "viewer",
  member_count: 3,
  size_bytes: 1536,
};

type ServerOptions = {
  role?: Workspace["role"];
  list?: OwnerlessNotebook[];
  /** The audit's pages by cursor, "" the first. */
  pages?: Record<string, NotebookAuditEventPage>;
  answers?: Record<string, Answer>;
};

/**
 * The server of Lab's ownerless page, as Ada (whose role in Lab is role)
 * sees it: list is ownerless, and the audit answers pages, to an admin
 * (else the reads named in forbids answer 403); taking over and deleting change the list, a notebook taken
 * over joining Ada's notebooks, except where answers answers otherwise.
 * What went out is in sent; the test changes the state as another tab
 * would.
 */
function ownerlessServer({ role = "admin", list = [ownerlessJSON, atlas], pages, answers = {} }: ServerOptions = {}) {
  const server = {
    sent: [] as string[],
    role,
    list,
    down: false,
    pages: pages ?? { "": { data: [auditEventJSON], next_cursor: null } },
    notebooks: [] as Notebook[],
    forbids: ["list", "audit"] as ("list" | "audit")[],
  };
  const admins = (kind: "list" | "audit", read: () => Response | Promise<Response>) =>
    server.role !== "admin" && server.forbids.includes(kind) ? problem(403, "forbidden") : read();
  const byId = (request: Request) => server.list.find((n) => new URL(request.url).pathname.includes(n.id));
  const app = signedInApp({
    "GET /api/v0/workspaces": () => {
      server.sent.push("GET workspaces");
      return json({ data: [{ ...workspaceJSON, role: server.role }] });
    },
    "GET /api/v0/workspaces/lab/notebooks": () => json({ data: server.notebooks }),
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => {
      server.sent.push("GET ownerless");
      return server.down ? Promise.reject(new TypeError("offline")) : admins("list", () => json({ data: server.list }));
    },
    "GET /api/v0/workspaces/lab/notebook-audit-events": (request) => {
      const cursor = new URL(request.url).searchParams.get("cursor") ?? "";
      server.sent.push(`GET audit ${cursor}`.trim());
      const answer = server.pages[cursor];
      return answer === undefined ? Promise.reject(new TypeError("offline")) : admins("audit", () => json(answer));
    },
    "POST /api/v0/ownerless-notebooks/*/take-over": async (request) => {
      const notebook = byId(request);
      server.sent.push(`take over ${notebook?.name}`);
      // Its admin from then on, Ada joins its members (take_over.go counts them after).
      const taken = notebook && {
        ...notebookJSON,
        id: notebook.id,
        name: notebook.name,
        workspace_access: notebook.workspace_access,
        role: "admin" as const,
        member_count: notebook.member_count + 1,
      };
      const answer = (await answers["take over"]?.(request)) ?? json(taken);
      if (answer.ok && taken) {
        server.list = server.list.filter((n) => n !== notebook);
        server.notebooks = [...server.notebooks, taken];
      }
      return answer;
    },
    "DELETE /api/v0/ownerless-notebooks/*": async (request) => {
      const notebook = byId(request);
      server.sent.push(`delete ${notebook?.name}`);
      const answer = (await answers.delete?.(request)) ?? new Response(null, { status: 204 });
      if (answer.ok) {
        server.list = server.list.filter((n) => n !== notebook);
      }
      return answer;
    },
  });
  return Object.assign(server, { app });
}

const heading = () => screen.findByRole("heading", { level: 2, name: "Ownerless notebooks" });

/** How the row's controls name a notebook of Bob's: by its name and its former owner. */
const takeOver = (name: string) => `Take over ${name}, former owner Bob (bob@example.com)`;
const deleteOne = (name: string) => `Delete ${name}, former owner Bob (bob@example.com)`;

/** The rows of the ownerless list, each as its lines. */
async function rows(): Promise<string[][]> {
  const list = await screen.findByRole("list", { name: "Ownerless notebooks" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => [...item.querySelectorAll("p")].map((p) => p.textContent ?? ""));
}

test("an admin reads the ownerless notebooks, the longest ownerless first, each with what it is and whose it was", async () => {
  renderApp(page, ownerlessServer().app);

  expect(await rows()).toEqual([
    [
      "Roadmap",
      "Former owner: Bob (bob@example.com)",
      "Private · Members left: 1 · Ownerless since Oct 2, 2026 · Last activity Oct 2, 2026 · Size 0 B",
    ],
    [
      "Atlas",
      "Former owner: Bob (bob@example.com)",
      "Workspace can read · Members left: 3 · Ownerless since Oct 2, 2026 · Last activity Oct 2, 2026 · Size 1.5 KB",
    ],
  ]);
  const nav = screen.getByRole("navigation", { name: "Workspace settings" });
  expect(
    within(nav)
      .getAllByRole("link")
      .map((link) => [link.textContent, link.getAttribute("aria-current")])
  ).toEqual([
    ["General", null],
    ["Members", null],
    ["Ownerless notebooks", "page"],
  ]);
});

test("with none, the page says so", async () => {
  renderApp(page, ownerlessServer({ list: [] }).app);

  expect(await screen.findByText("No ownerless notebooks.")).toBeTruthy();
});

test.each(["member", "guest"] as const)(
  "a %s is not offered the page; opened, it says whose it is, asking nothing",
  async (role) => {
    const server = ownerlessServer({ role });
    const { router } = renderApp("/lab/settings/general", server.app);
    const nav = await screen.findByRole("navigation", { name: "Workspace settings" });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((link) => link.textContent)
    ).toEqual(["General", "Members"]);

    await act(() => router.navigate(page));

    expect(await screen.findByText("Only the workspace's admins see its ownerless notebooks.")).toBeTruthy();
    expect(server.sent.filter((sent) => !sent.startsWith("GET workspaces"))).toEqual([]);
  }
);

test("taking one over puts it among the account's notebooks, says so with a way to it, and gives the focus to the heading", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer();
  const { router } = renderApp(page, server.app);

  // Taken over, Roadmap has its member left and the account: a team notebook.
  await user.click(await screen.findByRole("button", { name: takeOver("Roadmap") }));

  await waitFor(async () => expect(document.activeElement).toBe(await heading()));
  expect((await rows()).map(([name]) => name)).toEqual(["Atlas"]);
  const status = screen.getByRole("status");
  expect(status.textContent).toBe("Roadmap taken over. Open it");
  const nav = screen.getByRole("navigation", { name: "Lab" });
  expect(within(within(nav).getByRole("list", { name: "Team notebooks" })).getByRole("link").textContent).toBe(
    "Roadmap"
  );
  await waitFor(() => expect(server.sent.filter((sent) => sent === "GET audit")).toHaveLength(2));

  await user.click(within(status).getByRole("link", { name: "Open it" }));
  expect(await screen.findByRole("heading", { level: 1, name: "Roadmap" })).toBeTruthy();
  expect(router.state.location.pathname).toBe(`/lab/notebooks/${ownerlessJSON.id}`);
});

test("a take-over goes out once, however often it is pressed", async () => {
  const user = userEvent.setup();
  let answer: ((response: Response) => void) | undefined;
  const server = ownerlessServer({
    answers: { "take over": () => new Promise<Response>((resolve) => (answer = resolve)) },
  });
  renderApp(page, server.app);
  const button = await screen.findByRole("button", { name: takeOver("Roadmap") });

  await user.dblClick(button);
  await user.click(button);
  answer?.(json({ ...notebookJSON, id: ownerlessJSON.id, name: "Roadmap" }));

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Roadmap taken over. Open it"));
  expect(server.sent.filter((sent) => sent.startsWith("take over"))).toEqual(["take over Roadmap"]);
});

test("deleting one asks for its name, then the row leaves, the focus on the heading, and the audit is read again", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer();
  renderApp(page, server.app);

  await user.click(await screen.findByRole("button", { name: deleteOne("Atlas") }));
  const dialog = await screen.findByRole("alertdialog", { name: "Delete Atlas?" });
  expect(within(dialog).getByRole("button", { name: "Delete" })).toHaveProperty("disabled", true);
  await user.type(within(dialog).getByLabelText("Type Atlas to confirm"), "Atlas{Enter}");

  await waitFor(async () => expect(document.activeElement).toBe(await heading()));
  expect((await rows()).map(([name]) => name)).toEqual(["Roadmap"]);
  expect(server.sent).toContain("delete Atlas");
  await waitFor(() => expect(server.sent.filter((sent) => sent === "GET audit")).toHaveLength(2));
});

// Taken over, deleted or returned elsewhere: what was asked did not
// happen; the page says why, the row leaves, and the list, the audit and
// the workspaces are read again.
test.each([
  [
    "taking it over",
    "take over",
    async (user: ReturnType<typeof userEvent.setup>) => {
      await user.click(await screen.findByRole("button", { name: takeOver("Roadmap") }));
    },
  ],
  [
    "deleting it",
    "delete",
    async (user: ReturnType<typeof userEvent.setup>) => {
      await user.click(await screen.findByRole("button", { name: deleteOne("Roadmap") }));
      await user.type(await screen.findByLabelText("Type Roadmap to confirm"), "Roadmap{Enter}");
    },
  ],
])("%s once it is ownerless no more says so above the list", async (_, route, act_) => {
  const user = userEvent.setup();
  // Taken over elsewhere a moment before: no longer listed.
  const server = ownerlessServer({
    answers: {
      [route]: () => {
        server.list = [atlas];
        return problem(404, "notebook.not_found");
      },
    },
  });
  renderApp(page, server.app);

  await act_(user);

  expect((await screen.findByRole("alert")).textContent).toBe(
    "This notebook is ownerless no more: it was taken over, deleted, or returned to its former owner."
  );
  expect((await rows()).map(([name]) => name)).toEqual(["Atlas"]);
  await waitFor(async () => expect(document.activeElement).toBe(await heading()));
  await waitFor(() =>
    expect(server.sent.filter((sent) => ["GET ownerless", "GET audit", "GET workspaces"].includes(sent))).toEqual([
      "GET workspaces",
      "GET ownerless",
      "GET audit",
      "GET ownerless",
      "GET audit",
      "GET workspaces",
    ])
  );
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

// The server answers not found too to an account no longer the
// workspace's admin (ownerless.go): the page follows its role.
test("a take-over answered not found once the account is no longer an admin: the page says whose it is", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer({
    answers: {
      "take over": () => {
        server.role = "member";
        return problem(404, "notebook.not_found");
      },
    },
  });
  renderApp(page, server.app);

  await user.click(await screen.findByRole("button", { name: takeOver("Roadmap") }));

  expect(await screen.findByText("Only the workspace's admins see its ownerless notebooks.")).toBeTruthy();
});

test.each(["list", "audit"] as const)(
  "the %s read as forbidden reads the workspaces again: the page says whose it is",
  async (kind) => {
    const server = ownerlessServer();
    server.forbids = [kind];
    const { router } = renderApp("/lab/settings/general", server.app);
    await screen.findByRole("navigation", { name: "Workspace settings" });

    // Another admin made Ada a member; this tab has not read it yet.
    server.role = "member";
    await act(() => router.navigate(page));

    expect(await screen.findByText("Only the workspace's admins see its ownerless notebooks.")).toBeTruthy();
  }
);

test("the list says so when it cannot be read; Try again reads it", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer();
  server.down = true;
  renderApp(page, server.app);
  const section = (await heading()).closest("section") as HTMLElement;

  expect((await within(section).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.down = false;
  await user.click(within(section).getByRole("button", { name: "Try again" }));

  expect(await rows()).toHaveLength(2);
});

afterEach(() => vi.useRealTimers());

test("the list and the audit read again show what changed elsewhere", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = ownerlessServer();
  const { router } = renderApp(page, server.app);
  expect(await rows()).toHaveLength(2);

  server.list = [atlas];
  server.pages = {
    "": { data: [{ ...auditEventJSON, id: "e2", action: "deleted" }, auditEventJSON], next_cursor: null },
  };
  await act(() => router.navigate("/lab/settings/general"));
  await act(() => vi.advanceTimersByTimeAsync(2_000));
  await act(() => router.navigate(page));

  await waitFor(async () => expect(await rows()).toHaveLength(1));
  await waitFor(async () => expect(await events()).toHaveLength(2));
});

/** The audit's events listed, each as its sentence. */
async function events(): Promise<string[]> {
  const list = await screen.findByRole("list", { name: "Audit log" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => item.querySelector("p")?.textContent ?? "");
}

const event = (id: string, action: NotebookAuditEvent["action"]): NotebookAuditEvent => ({
  ...auditEventJSON,
  id,
  action,
});

test("the audit says what was done, the newest first, a page at a time", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer({
    pages: {
      "": { data: [event("e3", "taken_over"), event("e2", "deleted")], next_cursor: "c1" },
      c1: { data: [event("e1", "returned")], next_cursor: null },
    },
  });
  renderApp(page, server.app);

  expect(await events()).toEqual([
    "Ada took over Roadmap (former owner Bob).",
    "Ada deleted Roadmap (former owner Bob).",
  ]);
  await user.click(screen.getByRole("button", { name: "Load more" }));

  await waitFor(async () =>
    expect(await events()).toEqual([
      "Ada took over Roadmap (former owner Bob).",
      "Ada deleted Roadmap (former owner Bob).",
      "Roadmap was returned to Bob, back in the workspace.",
    ])
  );
  expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  const time = within(await screen.findByRole("list", { name: "Audit log" }))
    .getAllByRole("listitem")[0]
    ?.querySelector("time");
  expect(time?.getAttribute("dateTime")).toBe(auditEventJSON.created_at);
});

test("a page that cannot be loaded says why beside Load more, which tries again", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer({ pages: { "": { data: [event("e2", "deleted")], next_cursor: "c1" } } });
  renderApp(page, server.app);
  await events();

  await user.click(screen.getByRole("button", { name: "Load more" }));
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.pages = { ...server.pages, c1: { data: [event("e1", "returned")], next_cursor: null } };
  await user.click(screen.getByRole("button", { name: "Load more" }));

  await waitFor(async () => expect(await events()).toHaveLength(2));
  expect(screen.queryByRole("alert")).toBeNull();
});

test("an empty audit says so; one that cannot be read says why, and Try again reads it", async () => {
  const user = userEvent.setup();
  const server = ownerlessServer({ pages: {} });
  renderApp(page, server.app);
  const section = (await screen.findByRole("heading", { level: 2, name: "Audit log" })).closest(
    "section"
  ) as HTMLElement;

  expect((await within(section).findByRole("alert")).textContent).toBe(
    "Cannot reach the server. Check the connection and try again."
  );
  server.pages = { "": { data: [], next_cursor: null } };
  await user.click(within(section).getByRole("button", { name: "Try again" }));

  expect(await within(section).findByText("Nothing yet.")).toBeTruthy();
});

test("going from one workspace's ownerless notebooks straight to another's lists the other's, and its audit", async () => {
  const acme: Workspace = { ...workspaceJSON, id: "0199a2b4-0000-7000-8000-0000000000a1", slug: "acme", name: "Acme" };
  const app = signedInApp({
    "GET /api/v0/workspaces": () => json({ data: [acme, workspaceJSON] }),
    "GET /api/v0/workspaces/lab/ownerless-notebooks": () => json({ data: [ownerlessJSON] }),
    "GET /api/v0/workspaces/acme/ownerless-notebooks": () => json({ data: [atlas] }),
    "GET /api/v0/workspaces/lab/notebook-audit-events": () =>
      json({ data: [event("e1", "taken_over")], next_cursor: null }),
    "GET /api/v0/workspaces/acme/notebook-audit-events": () =>
      json({ data: [event("e2", "deleted")], next_cursor: null }),
  });
  const { router } = renderApp(page, app);
  expect((await rows()).map(([name]) => name)).toEqual(["Roadmap"]);
  expect(await events()).toEqual(["Ada took over Roadmap (former owner Bob)."]);

  await act(() => router.navigate("/acme/settings/ownerless"));

  await waitFor(async () => expect((await rows()).map(([name]) => name)).toEqual(["Atlas"]));
  await waitFor(async () => expect(await events()).toEqual(["Ada deleted Roadmap (former owner Bob)."]));
});
