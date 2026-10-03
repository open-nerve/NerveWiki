import { expect, test } from "vitest";

import { highlightBlock } from "./highlight-block";
import { highlightMarkup } from "./highlight-markup";

test("highlight.js's answer is taken, its text the block's", () => {
  const text = 'class A { f() { return `a${"<b>"}` } }';
  const { html } = highlightBlock({ id: 0, language: "javascript", text });

  const markup = highlightMarkup(html ?? "", text);

  expect(markup?.textContent).toBe(text);
  expect(markup?.querySelector("span.hljs-keyword")?.textContent).toBe("class");
  expect(markup?.querySelector("span.hljs-title.class_")).toBeTruthy();
});

test.each([
  ["text alone", "a &lt; b", "a < b"],
  ["a sub-scope's part", '<span class="hljs-title function_">f</span>', "f"],
  ["a language embedded in another", '<span class="language-css">p</span>', "p"],
])("%s is taken", (_name, html, text) => {
  expect(highlightMarkup(html, text)?.textContent).toBe(text);
});

test.each([
  ["a link", '<a href="https://example.com">a</a>', "a"],
  ["an image", '<img src="x" alt="">', ""],
  ["a span with another attribute", '<span class="hljs-keyword" onclick="x()">a</span>', "a"],
  ["a span with a style", '<span style="color: red">a</span>', "a"],
  ["a span with another class", '<span class="hljs-keyword nw-x">a</span>', "a"],
  ["a link with a class of highlight.js's", '<a class="hljs-keyword">a</a>', "a"],
  ["a span inside another element", '<b><span class="hljs-keyword">a</span></b>', "a"],
  ["another element inside a span", '<span class="hljs-string"><b>a</b></span>', "a"],
  ["a span with another attribute inside a span", '<span class="hljs-string"><span onclick="x()">a</span></span>', "a"],
  ["text other than the block's", '<span class="hljs-keyword">b</span>', "a"],
])("%s is refused", (_name, html, text) => {
  expect(highlightMarkup(html, text)).toBeUndefined();
});
