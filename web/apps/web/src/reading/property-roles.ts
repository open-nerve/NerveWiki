import type { Enhancement } from "./enhancement";

/** The roles a properties table keeps as the strip lays it out otherwise (decision 864), by what has each. */
const roles: [selector: string, role: string][] = [
  ["table.nw-props", "table"],
  ["table.nw-props > tbody", "rowgroup"],
  ["table.nw-props tr", "row"],
  ["table.nw-props th", "rowheader"],
  ["table.nw-props td", "cell"],
  ["table.nw-props td > ul", "list"],
  ["table.nw-props td > ul > li", "listitem"],
];

/**
 * propertyRoles gives the page's properties table, a nested one's too, the roles of a table and its list values a
 * list's (M13/P4/S1, decision 864): the strip (reading.css) sets their display otherwise, and WebKit with VoiceOver
 * drops the semantics of a table or a list displayed so. An element with a role keeps it; the server writes none,
 * and another table (an author's) is not in a property's value. Cleaned up, the roles it gave go: the view compares
 * blocks by the server's HTML, and those it writes anew have them again as it runs once more.
 */
export const propertyRoles: Enhancement = (container) => {
  const given: Element[] = [];
  for (const [selector, role] of roles) {
    for (const element of container.querySelectorAll(selector)) {
      if (!element.hasAttribute("role")) {
        element.setAttribute("role", role);
        given.push(element);
      }
    }
  }
  return () => {
    for (const element of given) {
      element.removeAttribute("role");
    }
  };
};
