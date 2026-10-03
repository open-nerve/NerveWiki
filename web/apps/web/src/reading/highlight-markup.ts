/**
 * The classes highlight.js gives its spans: a scope (hljs-keyword,
 * hljs-built_in), a sub-scope's part, which ends in an underscore for its
 * second level and two for its third (title.function.invoke is
 * hljs-title function_ invoke__), a language embedded in another
 * (language-css). None is a class of the app's: those have no underscore
 * at their end, and the reading view's own begin with nw-.
 */
const allowedClass = /^(?:hljs-[a-z_-]+|[a-z]+_{1,2}|language-[a-z0-9_+#.-]+)$/;

/**
 * highlightMarkup is html, highlight.js's answer for a code block of text,
 * as nodes to put in the block (M4/P5 design 3.9), or undefined unless it
 * is only text and span elements whose one attribute is a class of
 * highlight.js's, and its text is text: the front end does not trust what
 * highlight.js answers.
 */
export function highlightMarkup(html: string, text: string): DocumentFragment | undefined {
  const template = document.createElement("template");
  template.innerHTML = html;
  const fragment = template.content;
  for (const element of fragment.querySelectorAll("*")) {
    if (element.localName !== "span" || [...element.attributes].some((attribute) => attribute.name !== "class")) {
      return undefined;
    }
    if (![...element.classList].every((name) => allowedClass.test(name))) {
      return undefined;
    }
  }
  return fragment.textContent === text ? fragment : undefined;
}
