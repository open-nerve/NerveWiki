import { invertedEffects } from "@codemirror/commands";
import {
  MapMode,
  RangeSet,
  RangeValue,
  StateEffect,
  StateField,
  type EditorState,
  type Extension,
  type Range,
} from "@codemirror/state";

/** A line break as written: CRLF, CR alone, or LF. */
type LineBreak = "\r\n" | "\r" | "\n";

const byteOrderMark = String.fromCodePoint(0xfeff);
const anyBreak = /\r\n|\r|\n/g;

/**
 * Split is a content as the editor takes it (M4/P6 design 3.3): its text
 * without the byte order mark, every line break an LF; whether the mark
 * was there; the content's main line break, and each break as written.
 */
export type Split = {
  text: string;
  byteOrderMark: boolean;
  main: LineBreak;
  breaks: readonly LineBreak[];
};

/**
 * splitBreaks splits raw for the editor. The main line break is the one
 * written most, the first of those written as often, LF with none.
 */
export function splitBreaks(raw: string): Split {
  const marked = raw.startsWith(byteOrderMark);
  const breaks: LineBreak[] = [];
  const text = (marked ? raw.slice(byteOrderMark.length) : raw).replace(anyBreak, (written) => {
    breaks.push(written as LineBreak);
    return "\n";
  });
  return { text, byteOrderMark: marked, main: mainBreak(breaks), breaks };
}

function mainBreak(breaks: readonly LineBreak[]): LineBreak {
  const counts = new Map<LineBreak, number>();
  for (const written of breaks) {
    counts.set(written, (counts.get(written) ?? 0) + 1);
  }
  // A map iterates in the order its keys came: the first written wins a tie.
  let main: LineBreak = "\n";
  let most = 0;
  for (const [written, count] of counts) {
    if (count > most) {
      main = written;
      most = count;
    }
  }
  return main;
}

/**
 * A BreakMark is at the end of a line whose break is written other than
 * the main one. It stays at the line's end as text is typed or deleted
 * before it, and goes once the break itself is deleted: the character
 * after it (the editor's LF) is.
 */
class BreakMark extends RangeValue {
  constructor(readonly written: LineBreak) {
    super();
  }

  override eq(other: RangeValue): boolean {
    return other instanceof BreakMark && other.written === this.written;
  }
}
BreakMark.prototype.startSide = 1;
BreakMark.prototype.endSide = 1;
BreakMark.prototype.point = true;
BreakMark.prototype.mapMode = MapMode.TrackAfter;

const marks: Record<LineBreak, BreakMark> = {
  "\r\n": new BreakMark("\r\n"),
  "\r": new BreakMark("\r"),
  "\n": new BreakMark("\n"),
};

/** A break as written, at a line's end. */
type Written = { at: number; written: LineBreak };

/** restoreBreaks puts back the marks of breaks a change deleted, when it is undone. */
const restoreBreaks = StateEffect.define<readonly Written[]>({
  map: (breaks, mapping) => breaks.map(({ at, written }) => ({ at: mapping.mapPos(at, 1), written })),
});

type Breaks = { byteOrderMark: boolean; main: LineBreak; marks: RangeSet<BreakMark> };

const breaksField = StateField.define<Breaks>({
  create: () => ({ byteOrderMark: false, main: "\n", marks: RangeSet.empty }),
  update(value, tr) {
    let next = tr.docChanged ? value.marks.map(tr.changes) : value.marks;
    for (const effect of tr.effects) {
      if (effect.is(restoreBreaks)) {
        // The undo puts the breaks back where the effect says, mapped as they were.
        next = next.update({ add: effect.value.map(({ at, written }) => marks[written].range(at)), sort: true });
      }
    }
    return next === value.marks ? value : { ...value, marks: next };
  },
});

/**
 * lineBreaks keeps split's line breaks and byte order mark as the content
 * is edited (M4/P6 design 3.3): breaks typed or pasted are the main one;
 * a deleted break's undo brings back the one it was.
 */
export function lineBreaks(split: Split): Extension {
  const ranges: Range<BreakMark>[] = [];
  let at = -1;
  for (const written of split.breaks) {
    at = split.text.indexOf("\n", at + 1);
    if (written !== split.main) {
      ranges.push(marks[written].range(at));
    }
  }
  return [
    breaksField.init(() => ({
      byteOrderMark: split.byteOrderMark,
      main: split.main,
      marks: RangeSet.of(ranges),
    })),
    invertedEffects.of((tr) => {
      if (!tr.docChanged) {
        return [];
      }
      const deleted: Written[] = [];
      for (const cursor = tr.startState.field(breaksField).marks.iter(); cursor.value !== null; cursor.next()) {
        if (tr.changes.mapPos(cursor.from, 1, MapMode.TrackAfter) === null) {
          deleted.push({ at: cursor.from, written: cursor.value.written });
        }
      }
      return deleted.length === 0 ? [] : [restoreBreaks.of(deleted)];
    }),
  ];
}

/** joinBreaks writes state's content back: its byte order mark, and each line with its break as written. */
export function joinBreaks(state: EditorState): string {
  const { byteOrderMark: marked, main, marks: written } = state.field(breaksField);
  const parts: string[] = marked ? [byteOrderMark] : [];
  const cursor = written.iter();
  const { doc } = state;
  for (let number = 1; number <= doc.lines; number++) {
    const line = doc.line(number);
    parts.push(line.text);
    if (number < doc.lines) {
      while (cursor.value !== null && cursor.from < line.to) {
        cursor.next();
      }
      parts.push(cursor.value !== null && cursor.from === line.to ? cursor.value.written : main);
    }
  }
  return parts.join("");
}
