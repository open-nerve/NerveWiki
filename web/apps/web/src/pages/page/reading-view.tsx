import { reaction } from "mobx";
import { observer } from "mobx-react-lite";
import { useContext, useLayoutEffect, useRef, type RefObject } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { canEditPageBody } from "../../app/source-permissions";
import { NotLoaded } from "../../app/not-loaded";
import { Enhancements } from "../../reading/enhancement";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { useT } from "../../i18n/i18n";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { usePageView } from "./page-view";
import { useServerHTML, type Toggled } from "./server-html";
import { useUnresolvedLinks } from "./unresolved-link";
import { useViewAnchors } from "./view-anchors";

/**
 * ReadingView is the page's content as the server renders it (M4/P5 design
 * 3.8): HTML the server sanitized, which goes into the article as it is.
 * It is read by page, and read again as SWR does (a refocus, a retry):
 * another's write then shows. A 503 server_busy is read again after its
 * Retry-After.
 *
 * Once the HTML is in, the app's enhancements run on it in their order;
 * before the HTML is changed, and when the view goes, they are undone in
 * the reverse order (reading/enhancement.ts). A view read again changes
 * the HTML where it differs, block by block: what the reader selected,
 * opened or plays in the rest stays (M12/P2/S4, 3, 17.1). They run again as the
 * language changes, their names in it: the reader changes it in a menu,
 * which has the focus. Not as the theme does, the system's maybe as one
 * reads: those that draw in it follow it (onThemeChange). A writer's
 * tick task items through it, one of the page's at a time (the page tree store's
 * oneToggle), and refused tells the page what was refused, undefined as a
 * toggle starts (M5/P6 design 3.5). Whether the page can be written (the
 * account's role, an upgrade under way, a source) changes the checkboxes
 * alone, not the HTML, whose place the reader keeps (M12/P2/S4, 48). A task item's checkbox that had the
 * focus as the HTML changed has it back in the new HTML, without a
 * scroll, while the box at its position is the same item's (its text, and
 * its state, or the state the view's own toggle asked for): a tick does
 * not move it, a write that moved the items does. A folded callout it is
 * in opens again, as the reader had it.
 *
 * An enhancement goes to another address through the router: to a page,
 * whose heading takes the focus; or to an anchor. An address with an
 * anchor has the view go to the element it names, once the HTML is in,
 * once for each time the app goes to the address: the element shows and
 * takes the focus (M6/P3 design 6.7). The page keeps the last navigation
 * its view took in (anchored), as the view goes while the page is edited:
 * coming back from editing goes nowhere. When the anchor names no element
 * as the page opens (its first navigation, by a load or from another
 * page), unanchored gives the page's heading the focus if it is nowhere
 * (what had it went with the page before); and a view from the cache, which
 * may be older than the link, is read again: the element it brings shows
 * and takes the focus, if the reader has done nothing since (scrolled,
 * clicked, touched, pressed a key: the browser's own scrolling is none of
 * these, nor are an assistive technology's moves) and the focus is where it
 * was; a read after that one moves nothing. A link of the page to no
 * element leaves the focus where it is, and the page. What had the focus
 * as the HTML changed, the anchor's element among them, has it back: the
 * same element, or one of its key in the new HTML (a heading of its text,
 * not its id: one of its name put before it takes the id), shown again if
 * it showed and no longer does; else the block where it was, neither
 * shown nor opened (server-html.ts; M12/P2/S4, 4.4.3): a view read again
 * stays where it is.
 *
 * A link to a page that is not there, acted on, opens the view's dialog
 * (unresolved-link.tsx): a writer may create the page, where the server
 * says it would go (M6/P6 design 7).
 *
 * The attachments' addresses in the HTML expire: the view is read again
 * before they do (page-view.ts), and its enhancement keeps the media
 * playing as the HTML is replaced and signs a started one's address anew
 * as it fails, through the attachments' store (reading/assets.ts; M7/P4
 * design 4.5, 4.6).
 *
 * The HTML and its enhancements are useServerHTML's (server-html.ts), the
 * anchors useViewAnchors's (view-anchors.ts); the view gives them their
 * context and keeps its own toggle (M12/P2/S4, 14d).
 */
export const ReadingView = observer(function ReadingView({
  notebook,
  page,
  refused,
  unanchored,
  anchored,
}: {
  notebook: Notebook;
  page: TreeNode;
  refused: (error: unknown) => void;
  unanchored: () => void;
  // The last navigation the page's view took in, its anchor gone to: the location's key and fragment (one key may
  // have several: an address typed in).
  anchored: RefObject<string | undefined>;
}) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const assets = useAssets(notebook);
  const enhancements = useContext(Enhancements);
  const t = useT();
  const { preferences } = useStore();
  const navigate = useNavigate();
  const { data, error, mutate } = usePageView(notebook, page.id);
  // Whether the view came from the cache: older, maybe, than the address.
  const cached = useRef(data !== undefined);
  const article = useRef<HTMLElement>(null);
  // The view's own toggle, from its sending: the item's position and the state asked for, until a view shows it
  // or the toggle fails. Meanwhile the item may come in either state: another's write, read again, may come first.
  const toggled = useRef<Toggled | undefined>(undefined);
  // The page's latest refused: the HTML is not replaced for a new one.
  const latestRefused = useRef(refused);
  const latestUnanchored = useRef(unanchored);
  const links = useUnresolvedLinks({
    notebook,
    page: page.id,
    pages,
    article,
    reload: () => void mutate(),
    report: (failure) => latestRefused.current(failure),
  });
  const latestUnresolved = useRef(links.unresolved);
  const latestLeads = useRef(links.leads);
  // Before the view's HTML: the notebook's mode changing runs the enhancements again, which read the leads it makes.
  useLayoutEffect(() => {
    latestRefused.current = refused;
    latestUnanchored.current = unanchored;
    latestUnresolved.current = links.unresolved;
    latestLeads.current = links.leads;
  });
  const revision = data?.revision;
  useServerHTML(
    article,
    data && { html: data.html, revision: data.revision, assetsExpire: data.assets_expire_at ?? null },
    {
      enhancements,
      toggled,
      workspace: slug,
      notebook: notebook.id,
      page: page.id,
      t,
      locale: preferences.locale,
      mode: notebook.mode,
      theme: () => preferences.resolvedTheme,
      onThemeChange: (listener) => reaction(() => preferences.resolvedTheme, listener),
      reload: () => void mutate(),
      // An address without an anchor arrives at the page, whose heading takes the focus: the link had it.
      navigate: (to) => void navigate(to, to.includes("#") ? undefined : { state: arrived }),
      // Whether the page can be written is asked as an item is clicked, and as the toggle's turn comes.
      toggleTask: async (offset, checked) => {
        if (!canEditPageBody(pages, page.id)) return;
        await pages.oneToggle(page.id, async () => {
          if (!canEditPageBody(pages, page.id) || revision === undefined) return;
          latestRefused.current(undefined);
          toggled.current = { task: offset.toString(), checked };
          try {
            await pages.toggleTask(page.id, { base_revision: revision, offset, checked });
          } catch (failure) {
            toggled.current = undefined;
            if (failure instanceof ApiError && failure.code === "page.revision_mismatch") {
              await mutate();
            }
            throw failure;
          }
          await mutate();
        });
      },
      writes: () => canEditPageBody(pages, page.id),
      onWritesChange: (listener) => reaction(() => canEditPageBody(pages, page.id), listener),
      report: (failure) => latestRefused.current(failure),
      unresolved: (link) => void latestUnresolved.current(link),
      leads: (target) => latestLeads.current(target),
      assetAddress: (id) => assets.address(id),
    }
  );
  useViewAnchors({ article, html: data?.html, anchored, unanchored: latestUnanchored, cached, reload: mutate });
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  // Named by the page, a landmark: what is wider than it scrolls in its own region (reading/scroll-regions.ts).
  return (
    <>
      <article ref={article} aria-label={page.name} className="nw-reading min-w-0" />
      {links.dialog}
    </>
  );
});
