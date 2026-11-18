import { expect, type Locator, type Page } from "@playwright/test";

// Contrast as the page paints it (M12/P2/S3; WCAG 1.4.3, 1.4.11): the stories that measure it run on Chromium, whose
// canvas reads the oklch() and oklab() colours of the computed styles (M12/P2 design 4.3.1.3, 4.3.1.4).

/**
 * Contrast is WCAG 2's contrast ratio of what is in front against what is behind it, each as the page paints it:
 * painted on a canvas over what is under it and read back as 8-bit sRGB, as the browser composites (a translucent
 * ring over the page). What the canvas cannot read throws: no colour is guessed.
 */
export interface Contrast {
  ratio: number;
  front: string;
  back: string;
  /** What shows the focus: "outline solid 2px", "ring 3px", "ring 2px inset". */
  indicator?: string;
}

/** What measure measures: an element's text; the focus it has from the keyboard; the indicator it shows. */
type Measured = "text" | "focus" | "indicator";

/** textContrast is locator's text colour against its background and its ancestors', down to the first opaque one. */
export function textContrast(locator: Locator): Promise<Contrast> {
  return locator.evaluate(measure, "text" as const);
}

/**
 * focusContrast is the focus indicator of locator, which must have the focus as the keyboard gives it
 * (:focus-visible): its outline, or else the ring its box-shadow draws, against what is around it.
 */
export function focusContrast(locator: Locator): Promise<Contrast> {
  return locator.evaluate(measure, "focus" as const);
}

/**
 * indicatorContrast is focusContrast of an element that shows a focus without having it as the keyboard gives it:
 * the editor's frame, the current item of a menu or of a list box, whose ring is inset, against what is inside it.
 */
export function indicatorContrast(locator: Locator): Promise<Contrast> {
  return locator.evaluate(measure, "indicator" as const);
}

/** setColorScheme has page follow the system's scheme, and waits until it shows it (the dark class on html). */
export async function setColorScheme(page: Page, scheme: "light" | "dark"): Promise<void> {
  await page.emulateMedia({ colorScheme: scheme });
  const html = page.locator("html");
  await (scheme === "dark"
    ? expect(html).toHaveClass(/(^|\s)dark(\s|$)/)
    : expect(html).not.toHaveClass(/(^|\s)dark(\s|$)/));
}

// Serialized into the page by evaluate: it may use nothing outside itself.
function measure(element: Element, what: Measured): Contrast {
  // A control that has just taken the focus is in its transition's first frame (transition-[color,box-shadow]):
  // its own transitions and those under which it is painted end first (M12/P2 design review B-M2).
  for (let at: Element | null = element; at !== null; at = at.parentElement) {
    for (const animation of at.getAnimations()) {
      if (animation instanceof CSSTransition) animation.finish();
    }
  }
  const canvas = document.createElement("canvas");
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (context === null) throw new Error("no 2d canvas");
  const paint = (colours: string[]): number[] => {
    context.clearRect(0, 0, 1, 1);
    for (const colour of colours) {
      context.fillStyle = "#010203";
      context.fillStyle = colour;
      if (context.fillStyle === "#010203") throw new Error(`the canvas does not read ${colour}`);
      context.fillRect(0, 0, 1, 1);
    }
    return [...context.getImageData(0, 0, 1, 1).data];
  };
  const opaque = (colour: string) => paint([colour])[3] === 255;
  // The backgrounds from the first opaque one up to from, the order they are painted in.
  const under = (from: Element | null): string[] => {
    const colours: string[] = [];
    for (let at = from; at !== null; at = at.parentElement) {
      const colour = getComputedStyle(at).backgroundColor;
      colours.unshift(colour);
      if (opaque(colour)) return colours;
    }
    throw new Error("nothing opaque under it");
  };
  // oxlint-disable-next-line unicorn/consistent-function-scoping -- measure is serialized alone, its helpers in it
  const luminance = ([r = 0, g = 0, b = 0]: number[]) => {
    const [R = 0, G = 0, B = 0] = [r, g, b].map((c) => {
      const v = c / 255;
      return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * R + 0.7152 * G + 0.0722 * B;
  };
  // oxlint-disable-next-line unicorn/consistent-function-scoping -- measure is serialized alone, its helpers in it
  const hex = (p: number[]) =>
    `#${p
      .slice(0, 3)
      .map((c) => c.toString(16).padStart(2, "0"))
      .join("")}`;
  const style = getComputedStyle(element);
  let front: string;
  let back: string[];
  let indicator: string | undefined;
  if (what === "text") {
    back = under(element);
    front = style.color;
  } else {
    if (what === "focus" && !(element === document.activeElement && element.matches(":focus-visible"))) {
      throw new Error("it does not have the focus as the keyboard gives it");
    }
    if (style.outlineStyle !== "none" && Number.parseFloat(style.outlineWidth) > 0) {
      // An outline is drawn outside the element, over what is under it.
      back = under(element.parentElement);
      front = style.outlineColor;
      indicator = `outline ${style.outlineStyle} ${style.outlineWidth}`;
    } else {
      // Tailwind's ring: a shadow of no offset nor blur, spread out; its colour first, inset last when it is drawn
      // inside the element, over the element's own background.
      const shadows = style.boxShadow.split(/,(?![^(]*\))/).map((shadow) => shadow.trim());
      const ring = shadows
        .map((shadow) => shadow.match(/^(.*?)\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+(-?[\d.]+)px(\s+inset)?$/))
        .find((m) => m !== null && m[2] === "0" && m[3] === "0" && m[4] === "0" && Number(m[5]) > 0);
      if (ring === undefined || ring === null) throw new Error(`no focus indicator: ${style.boxShadow}`);
      const inset = ring[6] !== undefined;
      back = under(inset ? element : element.parentElement);
      front = ring[1] ?? "";
      indicator = `ring ${ring[5] ?? ""}px${inset ? " inset" : ""}`;
    }
  }
  const behind = paint(back);
  const painted = paint([...back, front]);
  const [lighter = 0, darker = 0] = [luminance(painted), luminance(behind)].toSorted((a, b) => b - a);
  return { ratio: (lighter + 0.05) / (darker + 0.05), front: hex(painted), back: hex(behind), indicator };
}
