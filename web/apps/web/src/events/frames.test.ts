import { describe, expect, test } from "vitest";

import { FrameParser } from "./frames";

describe("FrameParser", () => {
  test("parses frames that come in pieces, a heartbeat between them", () => {
    const parser = new FrameParser();

    const first = parser.push('event: hello\ndata: {"heartbe');
    const rest = parser.push(
      'at_seconds":20}\n\n: heartbeat\n\nevent: lock\ndata: {"workspace_id":"w","notebook_id":"n","page_id":"p","session_id":"s"}\n\n'
    );

    expect(first).toEqual([]);
    expect(rest).toEqual([
      { type: "hello", data: { heartbeat_seconds: 20 } },
      { type: "beat" },
      { type: "lock", data: { workspace_id: "w", notebook_id: "n", page_id: "p", session_id: "s" } },
    ]);
  });

  test("reads pages and reset, CRLF line ends and a field without its space", () => {
    const parser = new FrameParser();

    expect(
      parser.push(
        'event:pages\r\ndata:{"workspace_id":"w","notebook_id":"n","tree":true,"pages":null}\r\n\r\nevent: reset\ndata: {"reason":"access"}\n\n'
      )
    ).toEqual([
      { type: "pages", data: { workspace_id: "w", notebook_id: "n", tree: true, pages: null } },
      { type: "reset", data: { reason: "access" } },
    ]);
  });

  test("joins the lines of a frame's data; names an event it does not know without its data; drops a frame that is not JSON", () => {
    const parser = new FrameParser();

    expect(
      parser.push(
        'event: links\ndata: {"a":1}\n\nevent: lock\ndata: {"page_id":\ndata: "p"}\n\nevent: pages\ndata: {nope\n\n'
      )
    ).toEqual([
      { type: "other", event: "links" },
      { type: "lock", data: { page_id: "p" } },
    ]);
  });
});
