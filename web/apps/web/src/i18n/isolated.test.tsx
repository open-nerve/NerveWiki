import { render } from "@testing-library/react";
import { expect, test } from "vitest";

import { translator } from "./i18n";
import { isolate, Isolated } from "./isolated";

// A name in a line of other text is written in a bdi, where the translation puts it (M12/P2/S3; M5-polish 26.12,
// 32.4).

/** bdis are the texts of the bdis in element, in order. */
const bdis = (element: Element) => [...element.querySelectorAll("bdi")].map((bdi) => bdi.textContent);

test("Isolated writes each name in a bdi where the translation puts it, the text around it as it is, in English and in Chinese", () => {
  for (const [locale, said] of [
    ["en", "renamed “מדריך” to “התקנה”"],
    ["zh-CN", "改名「מדריך」为「התקנה」"],
  ] as const) {
    const t = translator(locale);
    const { container, unmount } = render(
      <Isolated {...isolate((name) => t("history.change.renamed", { from: name("מדריך"), to: name("התקנה") }))} />
    );
    expect(bdis(container), locale).toEqual(["מדריך", "התקנה"]);
    expect(container.textContent, locale).toBe(said);
    unmount();
  }
});

test("the parts a text is composed of give their names slots of their own", () => {
  const isolating = isolate((name) => [name("א"), name("ב")].join(" / "));
  expect(isolating.names).toEqual(["א", "ב"]);
  const { container } = render(<Isolated {...isolating} />);
  expect(bdis(container)).toEqual(["א", "ב"]);
  expect(container.textContent).toBe("א / ב");
});

test("a text of no names is written as it is", () => {
  const { container } = render(<Isolated {...isolate(() => "Client: CLI")} />);
  expect(bdis(container)).toEqual([]);
  expect(container.textContent).toBe("Client: CLI");
});
