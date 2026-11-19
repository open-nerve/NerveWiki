import { assetOf } from "./assets";
import type { Enhancement } from "./enhancement";

/**
 * expiredDownloads has a link of the view that downloads an attachment
 * (a.nw-asset[download]), clicked once the view's addresses have expired
 * (asleep, offline, a read again that failed), download from its
 * attachment's address signed anew, not fail unseen in the downloads' bar
 * (M12/P2/S5, 17.4): the click is held, the address signed anew
 * (assetAddress) and written to the link, which is clicked again; the view
 * is read again, once for each expiry. An address that cannot be signed
 * leaves the old one, which the browser says failed. A link that opens in
 * a tab of its own is not held: that opens in the click, or not at all
 * (13.2 item 27). A click with a key held, or not the main button's, goes
 * as the browser has it.
 *
 * The click it makes itself goes through once, as it is dispatched (click
 * dispatches at once): the link and this closure stay as the view's
 * addresses are signed anew in place (M12/P2/S4, 17.1), so a later click
 * compares the expiry again.
 */
export const expiredDownloads: Enhancement = (container, context) => {
  // The link clicked again, as that click is dispatched.
  let passing: HTMLAnchorElement | undefined;
  // The expiry the view was read again for.
  let reloaded: string | undefined;
  const onClick = (event: MouseEvent) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.altKey ||
      !(event.target instanceof Element)
    ) {
      return;
    }
    const link = event.target.closest<HTMLAnchorElement>("a.nw-asset[download][href]");
    if (link === null || link === passing) {
      return;
    }
    const expires = context.assetsExpire;
    const id = assetOf(link.getAttribute("href"));
    if (expires === null || id === undefined || Date.now() < Date.parse(expires)) {
      return;
    }
    event.preventDefault();
    void downloadAnew(link, id, expires);
  };
  const downloadAnew = async (link: HTMLAnchorElement, id: string, expires: string) => {
    try {
      link.setAttribute("href", await context.assetAddress(id));
    } catch {
      // Not signed (the attachment gone, or the network): the old address, which the browser says failed.
    }
    if (!link.isConnected) {
      return;
    }
    passing = link;
    try {
      link.click();
    } finally {
      passing = undefined;
    }
    if (reloaded !== expires) {
      reloaded = expires;
      context.reload();
    }
  };
  container.addEventListener("click", onClick);
  return () => container.removeEventListener("click", onClick);
};
