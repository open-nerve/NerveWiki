import type { ReactNode } from "react";

// Names in a line of other text (M12/P2 design 4.3.1.5; M5-polish 26.12, 32.4): a name of another direction, a page's
// in Hebrew say, beside the text around it (a date, a count, another name, an arrow) turns that text around it, the
// Unicode bidirectional algorithm's neutrals taking the name's direction. Each name is written in a bdi, where the
// translation puts it: en and zh-CN have their names at other places of a sentence ("Deleted by {name}", "{name} 删除").

/** Isolating is a translation whose names stand in it as slots, and the names, by their slots' indexes. */
export type Isolating = { text: string; names: readonly string[] };

/** A Namer gives a name its place in a text: as it is (same), or as a slot of an Isolating (isolate). */
export type Namer = (value: string) => string;

/** same writes a name as it is: where a text is a string only (an option, a label), it is the text of today. */
export const same: Namer = (value) => value;

// Private-use characters, which no translation has: a slot passes through Translate as any other value.
const slotted = /\uE000(\d+)\uE001/;

/**
 * isolate composes one Isolating: compose writes its text, each name in it given by name(value), which keeps the
 * value and answers the next slot. Every part the text is composed of takes this one name: no two names share a slot.
 */
export function isolate(compose: (name: Namer) => string): Isolating {
  const names: string[] = [];
  const text = compose((value) => `\uE000${(names.push(value) - 1).toString()}\uE001`);
  return { text, names };
}

/**
 * Isolated writes text with each slot's name in a bdi, where the translation puts it (UAX #9's isolates); nameClass,
 * the bdis' classes (the page's head shortens the name alone, M13/P4/S1, decision 899).
 */
export function Isolated({ text, names, nameClass }: Isolating & { nameClass?: string }): ReactNode {
  return text.split(slotted).map((part, i) => {
    if (i % 2 === 0) {
      return part;
    }
    const name = names[Number(part)];
    return (
      // oxlint-disable-next-line react/no-array-index-key -- a translation may write one name twice: by where it is
      <bdi key={i} className={nameClass}>
        {name}
      </bdi>
    );
  });
}
