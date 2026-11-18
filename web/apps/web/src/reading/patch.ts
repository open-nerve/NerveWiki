/**
 * The reading view's HTML changed in place (M12/P2/S4; M12/P2 design
 * 4.4.2): what the server's HTML read again changes of the view's, the
 * rest left as it is, its nodes the same.
 *
 * An attachment's content's address (asset/adapter/http's ContentURL)
 * is signed until the end of the hour after it is read: a view read again
 * in another hour differs in its addresses' expiry (e) and signature (s)
 * alone. Another file (b), a download (d=1) or another attachment is a
 * change of the view. An address written in the text is text: only the
 * src and href of an element are addresses.
 */

/** address is an attachment's content's address as the server spells it: its expiry and signature what changes. */
const address =
  /^(\/api\/v0\/assets\/[0-9a-f-]{36}\/content\?b=[0-9a-f-]{36}&e=)[1-9][0-9]*(&s=)[A-Za-z0-9_-]{22}(&d=1)?$/;

/** The attributes an address is in. */
const addressed = ["src", "href"] as const;

/** template is html parsed, as the view writes it: its content is the view's nodes. */
export function template(html: string): HTMLTemplateElement {
  const parsed = document.createElement("template");
  parsed.innerHTML = html;
  return parsed;
}

/** maskAddresses has each address of root and the elements in it, in src or href, say no expiry and no signature. */
function maskAddresses(root: Element | DocumentFragment): void {
  const elements = root.querySelectorAll("[src], [href]");
  for (const element of root instanceof Element ? [root, ...elements] : elements) {
    for (const name of addressed) {
      const value = element.getAttribute(name);
      if (value !== null && address.test(value)) {
        element.setAttribute(name, value.replace(address, "$1$2$3"));
      }
    }
  }
}

/** masked is template's HTML, its addresses masked. */
function masked(parsed: HTMLTemplateElement): string {
  const copy = parsed.cloneNode(true) as HTMLTemplateElement;
  maskAddresses(copy.content);
  return copy.innerHTML;
}

/**
 * addressesOnly tells whether after is before but for its attachments' addresses' expiry and signature, signed anew
 * (the same HTML too): the view's nodes may stay, their addresses replaced (resign).
 */
export function addressesOnly(before: HTMLTemplateElement, after: HTMLTemplateElement): boolean {
  return masked(before) === masked(after);
}
