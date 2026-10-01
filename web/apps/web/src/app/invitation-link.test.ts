import { expect, test } from "vitest";

import { invitationLink } from "./invitation-link";

// The link of an invitation (M2 design 4; M2/P6 design 3.3).

test.each([
  [
    "https://wiki.example.com",
    { id: "0199a2b4-0000-7000-8000-0000000000e1", token: "nwk_inv_AbC-_9" },
    "https://wiki.example.com/invitations/0199a2b4-0000-7000-8000-0000000000e1#nwk_inv_AbC-_9",
  ],
  ["http://192.168.1.5:8080", { id: "a b", token: "x#y" }, "http://192.168.1.5:8080/invitations/a%20b#x%23y"],
])("the link at %s carries the token in the fragment alone", (origin, link, expected) => {
  const url = new URL(invitationLink(origin, link));
  expect(url.href).toBe(expected);
  expect(url.search).toBe("");
});
