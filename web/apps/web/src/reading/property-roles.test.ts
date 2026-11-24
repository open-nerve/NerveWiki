import { afterEach, expect, test } from "vitest";

import { translator } from "../i18n/i18n";
import type { ReadingContext } from "./enhancement";
import { propertyRoles } from "./property-roles";

// The roles a properties table keeps as the strip lays it out otherwise (M13/P4/S1, decision 864): changed in
// display, WebKit with VoiceOver drops a table's and a list's.

afterEach(() => {
  document.body.replaceChildren();
});

const context: ReadingContext = {
  workspace: "lab",
  notebook: "n",
  page: "p",
  revision: 1,
  t: translator("en"),
  locale: "en",
  theme: () => "light",
  onThemeChange: () => () => undefined,
  reload: () => {},
  assetsExpire: null,
  assetAddress: () => Promise.reject(new Error("no attachments")),
  navigate: () => {},
  report: () => {},
  unresolved: () => {},
};

/** container holds html as the server writes it: its tables without tbody, which the parser adds. */
function container(html: string): HTMLElement {
  const element = document.createElement("article");
  element.innerHTML = html;
  document.body.append(element);
  return element;
}

/** rolesOf is each element of container's with a role, as "<tag> <role>", in the document's order. */
function rolesOf(element: HTMLElement): string[] {
  return [...element.querySelectorAll("[role]")].map(
    (each) => `${each.tagName.toLowerCase()} ${each.getAttribute("role") ?? ""}`
  );
}

const properties = [
  '<div class="nw-scroll"><table class="nw-props">',
  "<tr><th>status</th><td>draft</td></tr>",
  "<tr><th>tags</th><td><ul><li>alpha</li><li>beta</li></ul></td></tr>",
  '<tr><th>meta</th><td><table class="nw-props"><tr><th>a</th><td>1</td></tr></table></td></tr>',
  "</table></div>",
  "<table><tr><th>Name</th></tr><tr><td>Plain</td></tr></table>",
].join("");

test("a properties table, a nested one too, keeps a table's roles, its list values a list's; another table none", () => {
  const view = container(properties);

  propertyRoles(view, context);
  expect(rolesOf(view)).toEqual([
    "table table",
    "tbody rowgroup",
    "tr row",
    "th rowheader",
    "td cell",
    "tr row",
    "th rowheader",
    "td cell",
    "ul list",
    "li listitem",
    "li listitem",
    "tr row",
    "th rowheader",
    "td cell",
    "table table",
    "tbody rowgroup",
    "tr row",
    "th rowheader",
    "td cell",
  ]);
});

test("a role an element has is left as it is; cleaned up, those given go, those it had stay", () => {
  const view = container(properties.replace("<tr><th>status</th>", '<tr role="presentation"><th>status</th>'));

  const cleanup = propertyRoles(view, context);
  expect(view.querySelector("tr")?.getAttribute("role")).toBe("presentation");
  expect(view.querySelector("table")?.getAttribute("role")).toBe("table");

  cleanup?.();
  expect(rolesOf(view)).toEqual(["tr presentation"]);
});
