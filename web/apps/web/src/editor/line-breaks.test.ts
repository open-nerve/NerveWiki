import { history, redo, undo } from "@codemirror/commands";
import { EditorState, type TransactionSpec } from "@codemirror/state";
import { describe, expect, test } from "vitest";

import { joinBreaks, lineBreaks, splitBreaks } from "./line-breaks";

const bom = String.fromCodePoint(0xfeff);
const acute = String.fromCodePoint(0x0301);

/** An editor's state of raw, with its history. */
function editing(raw: string): EditorState {
  const split = splitBreaks(raw);
  return EditorState.create({ doc: split.text, extensions: [lineBreaks(split), history()] });
}

/** The state after each spec, one transaction each. */
function after(state: EditorState, ...specs: TransactionSpec[]): EditorState {
  return specs.reduce((current, spec) => current.update(spec).state, state);
}

/** The state after command, which must run. */
function run(state: EditorState, command: typeof undo): EditorState {
  let next = state;
  expect(command({ state, dispatch: (tr) => (next = tr.state) })).toBe(true);
  return next;
}

/** The end of the line number of state, as the editor counts it. */
const endOf = (state: EditorState, number: number) => state.doc.line(number).to;

describe("splitBreaks", () => {
  test("gives the editor LF alone, without the byte order mark", () => {
    expect(splitBreaks(`${bom}a\r\nb\rc\nd`)).toEqual({
      text: "a\nb\nc\nd",
      byteOrderMark: true,
      main: "\r\n",
      breaks: ["\r\n", "\r", "\n"],
    });
  });

  test("the main break is the one written most, the first of a tie, LF with none", () => {
    expect(splitBreaks("a\nb\r\nc\r\nd").main).toBe("\r\n");
    expect(splitBreaks("a\rb\nc").main).toBe("\r");
    expect(splitBreaks("a\nb\rc").main).toBe("\n");
    expect(splitBreaks("abc").main).toBe("\n");
  });

  test("reads CR before LF as one break, CR then CRLF as two", () => {
    expect(splitBreaks("a\r\r\nb").breaks).toEqual(["\r", "\r\n"]);
  });
});

describe("joinBreaks", () => {
  test.each([
    ["CRLF", "# Title\r\n\r\nLine one\r\nLine two\r\n"],
    ["CR alone", "a\rb\r\rc\r"],
    ["mixed", "a\r\nb\nc\rd"],
    ["CR then CRLF", "a\r\r\nb\n\r"],
    ["a byte order mark", `${bom}# Title\n`],
    ["only a byte order mark", bom],
    ["empty", ""],
    ["no break at the end", "a\nb"],
    ["trailing blanks", "line   \nnext\t\t\n   \n"],
    ["NFD", `Cafe${acute} and more\r\n`],
  ])("gives back an untouched content byte for byte: %s", (_, raw) => {
    expect(joinBreaks(editing(raw))).toBe(raw);
  });

  test("keeps the breaks of text typed at a line's end, at its start and after the byte order mark", () => {
    let state = editing(`${bom}a\r\nb\nc\rd`);
    state = after(state, { changes: { from: endOf(state, 2), insert: "X" } });
    state = after(state, { changes: { from: state.doc.line(3).from, insert: "Y" } });
    state = after(state, { changes: { from: 0, insert: "Z" } });
    expect(joinBreaks(state)).toBe(`${bom}Za\r\nbX\nYc\rd`);
  });

  test("keeps a break whose line's last characters are deleted", () => {
    let state = editing("ab\nc\r\nd\r\ne");
    state = after(state, { changes: { from: 1, to: 2 } });
    expect(joinBreaks(state)).toBe("a\nc\r\nd\r\ne");
  });

  test("writes a new line's break as the main one", () => {
    let state = editing("a\r\nb\r\nc\nd");
    state = after(state, { changes: { from: endOf(state, 3), insert: "\nnew" } });
    state = after(state, { changes: { from: endOf(state, 1), insert: "\n" } });
    expect(joinBreaks(state)).toBe("a\r\n\r\nb\r\nc\r\nnew\nd");
  });

  test("writes the breaks of pasted lines as the main one, however the paste wrote them", () => {
    let state = editing("a\nb");
    state = after(state, { changes: { from: 1, insert: "1\r\n2\r3" } });
    expect(joinBreaks(state)).toBe("a1\n2\n3\nb");
  });

  test("a break deleted from the start of the next line goes; the joined line keeps the next one's", () => {
    let state = editing("a\r\nb\nc\rd");
    const start = state.doc.line(3).from;
    state = after(state, { changes: { from: start - 1, to: start } });
    expect(joinBreaks(state)).toBe("a\r\nbc\rd");
  });

  test("a deleted break leaves nothing behind: a line ending later where it was keeps its own break", () => {
    let state = editing("x\ny\nb\r\nc\nd");
    const start = state.doc.line(4).from;
    state = after(state, { changes: { from: start - 1, to: start } }, { changes: { from: start - 1, to: start } });
    expect(joinBreaks(state)).toBe("x\ny\nb\nd");
  });

  test("a selection over breaks takes them with it", () => {
    let state = editing("a\rb\nc\rd\ne");
    state = after(state, { changes: { from: 1, to: state.doc.line(4).from } });
    expect(joinBreaks(state)).toBe("ad\ne");
  });
});

describe("undo", () => {
  test("brings back a deleted break as it was written; redo deletes it again", () => {
    const raw = "a\r\nb\nc\rd";
    let state = editing(raw);
    const start = state.doc.line(3).from;
    state = after(state, { changes: { from: start - 1, to: start } });
    state = run(state, undo);
    expect(joinBreaks(state)).toBe(raw);
    state = run(state, redo);
    expect(joinBreaks(state)).toBe("a\r\nbc\rd");
    state = run(state, undo);
    expect(joinBreaks(state)).toBe(raw);
  });

  test("brings back the breaks of a selection deleted, after later typing elsewhere", () => {
    const raw = "a\rb\nc\rd\ne";
    let state = editing(raw);
    state = after(state, { changes: { from: 1, to: state.doc.line(4).from } });
    state = after(state, { changes: { from: 0, insert: "Z" } });
    state = run(state, undo);
    state = run(state, undo);
    expect(joinBreaks(state)).toBe(raw);
  });
});
