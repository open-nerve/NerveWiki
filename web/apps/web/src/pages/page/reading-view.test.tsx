import { act, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";

import { routes } from "../../app/routes";
import type { Enhancement } from "../../reading/enhancement";
import { notebookJSON } from "../../test/fakes";
import { install, pagePath, pageServer } from "../../test/page-server";
import { renderApp } from "../../test/render";

// The reading view's enhancements (M4/P5 design 3.8).

afterEach(() => vi.useRealTimers());

/** recording is an enhancement that logs, as name, the HTML it runs on and what it knows, and the HTML it is undone on. */
function recording(log: string[], name: string): Enhancement {
  return (container, context) => {
    const { workspace, notebook, page, revision, role } = context;
    const known = notebook === notebookJSON.id && page === install.id ? "Install" : "?";
    log.push(`${name} on ${container.innerHTML}: ${workspace} ${known} ${revision} ${role}`);
    return () => log.push(`undo ${name} on ${container.innerHTML}`);
  };
}

test("the enhancements run on the HTML in their order, undone in the reverse before it is replaced and as the page goes", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const server = pageServer({ role: "editor" });
  const log: string[] = [];
  const { router } = renderApp(pagePath(install.id), server.app, routes, [recording(log, "a"), recording(log, "b")]);

  await waitFor(() => expect(log).toHaveLength(2));
  expect(log).toEqual(["a on <p>Install</p>: lab Install 1 editor", "b on <p>Install</p>: lab Install 1 editor"]);

  server.views.set(install.id, { html: "<p>Install, changed</p>", revision: 2 });
  await act(() => vi.advanceTimersByTimeAsync(6_000));
  act(() => void window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(log).toHaveLength(6));
  expect(log.slice(2)).toEqual([
    "undo b on <p>Install</p>",
    "undo a on <p>Install</p>",
    "a on <p>Install, changed</p>: lab Install 2 editor",
    "b on <p>Install, changed</p>: lab Install 2 editor",
  ]);

  await act(() => router.navigate(`/lab/notebooks/${notebookJSON.id}`));
  expect(log.slice(6)).toEqual(["undo b on <p>Install, changed</p>", "undo a on <p>Install, changed</p>"]);
});

const throws: Enhancement = () => {
  throw new Error("enhancement");
};

test("an enhancement that throws leaves the page and the others be", async () => {
  const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
  const log: string[] = [];
  renderApp(pagePath(install.id), pageServer().app, routes, [throws, recording(log, "b")]);

  expect((await screen.findByRole("article")).innerHTML).toBe("<p>Install</p>");
  await waitFor(() => expect(log).toEqual(["b on <p>Install</p>: lab Install 1 admin"]));
  expect(error).toHaveBeenCalledTimes(1);
});

test("an enhancement reads the view again through its context", async () => {
  const server = pageServer();
  const reloads: (() => void)[] = [];
  renderApp(pagePath(install.id), server.app, routes, [
    (_container, context) => {
      reloads.push(context.reload);
      return undefined;
    },
  ]);
  await waitFor(() => expect(reloads).toHaveLength(1));
  server.views.set(install.id, { html: "<p>Install, again</p>", revision: 2 });

  act(() => reloads[0]?.());

  await waitFor(() => expect(screen.getByRole("article").innerHTML).toBe("<p>Install, again</p>"));
  expect(server.sent.filter((line) => line === "GET view Install")).toHaveLength(2);
});
