import { useRef, useState, type ReactNode, type RefObject } from "react";
import { useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { ConfirmDialog } from "../../app/confirm-dialog";
import { writesPages } from "../../app/effective-role";
import type { HeldDialog } from "../../app/held-dialog";
import { useMounted } from "../../app/mounted";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from "../../components/ui/alert-dialog";
import { Button } from "../../components/ui/button";
import { useT, type Translate } from "../../i18n/i18n";
import type { UnresolvedLink } from "../../reading/enhancement";
import { ApiError } from "../../services/api";
import type { LinkLanding } from "../../services/linking.service";
import type { Notebook } from "../../services/notebook.service";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { useWorkspace } from "../workspace/workspace-layout";

type Landing = NonNullable<LinkLanding["landing"]>;

/** Why a link's page is not created: the reader cannot, an embed or an image does not, or the server's reason. */
type Why = "reader" | "embed" | "image" | NonNullable<LinkLanding["reason"]>;

/**
 * What the dialog asks or says, of the link it opened for, which was at of
 * the view's links to the same target; each opening has its own n.
 */
type Asked = { n: number; link: UnresolvedLink; at: number } & ({ landing: Landing } | { why: Why });

/**
 * useUnresolvedLinks is how the reading view of the page id answers a
 * link to a page that is not there, acted on (M6/P6 design 7): unresolved
 * opens the dialog that dialog renders.
 *
 * To a writer of the notebook's pages, the view asks the server where a
 * page made for the link would go, the link busy meanwhile, one link at a
 * time. A page the link leads to by now is gone to; a landing is a
 * confirmation, Create page "title" under its parent or at the top level,
 * whose confirm creates the page there and goes to it, arrived at, the
 * view read again for when the reader comes back. A title taken in the
 * meantime asks again where the page would go: a page the link leads to by
 * then is gone to; else the dialog says why. No landing, an embed's link,
 * an image's, and any link to a reader, say why it is not there, without a
 * question to the server for those. What the server refuses otherwise
 * goes to the page (report), as a refusal of Edit does, and the next
 * question clears it. A page gone to that the tree does not have yet,
 * another tab's, is read with the tree first.
 *
 * Closed without going, the dialog gives the focus back to its link, or,
 * read again since, to the link at the same place among those to its
 * target, or to the article.
 */
export function useUnresolvedLinks({
  notebook,
  page,
  pages,
  article,
  reload,
  report,
}: {
  notebook: Notebook;
  page: string;
  pages: PageTreeStore;
  article: RefObject<HTMLElement | null>;
  reload: () => void;
  report: (error: unknown) => void;
}): { unresolved: (link: UnresolvedLink) => Promise<void>; dialog: ReactNode } {
  const t = useT();
  const { slug } = useWorkspace();
  const navigate = useNavigate();
  const mounted = useMounted();
  const [asked, setAsked] = useState<Asked>();
  const [open, setOpen] = useState(false);
  const asking = useRef(false);
  const openings = useRef(0);

  // The page id, created by now, goes to it; a page another tab made that this tab's tree does not have yet is read
  // first, or the page would not be found.
  const go = async (id: string) => {
    if (pages.byId(id) === undefined) {
      await pages.load().catch(() => undefined);
    }
    if (mounted()) {
      void navigate(`/${slug}/notebooks/${notebook.id}/pages/${id}`, { state: arrived });
    }
  };
  async function unresolved(link: UnresolvedLink): Promise<void> {
    if (asking.current) {
      return;
    }
    const at = sameTarget(article.current, link.target).indexOf(link.element);
    const show = (answer: { landing: Landing } | { why: Why }) => {
      setAsked({ n: ++openings.current, link, at, ...answer });
      setOpen(true);
    };
    if (link.kind !== "link" || !writesPages(notebook.role)) {
      show({ why: link.kind === "link" ? "reader" : link.kind });
      return;
    }
    asking.current = true;
    link.element.setAttribute("aria-busy", "true");
    // A refusal said before is the last one's no longer, as Edit's and a tick's are not.
    report(undefined);
    try {
      const answer = await pages.landing(page, link.target);
      if (answer.node_id !== null) {
        await go(answer.node_id);
      } else if (answer.landing === null) {
        show({ why: answer.reason ?? "target_invalid" });
      } else {
        show({ landing: answer.landing });
      }
    } catch (error) {
      // A target longer than the server takes has no landing, nor one past a proxy's limit on the address (414).
      if (error instanceof ApiError && (error.code === "validation_failed" || error.status === 414)) {
        show({ why: "target_invalid" });
      } else {
        report(error);
      }
    } finally {
      asking.current = false;
      link.element.removeAttribute("aria-busy");
    }
  }

  async function create(target: string, landing: Landing): Promise<void> {
    let id: string;
    try {
      id = await pages.create(landing.parent_id, landing.title);
    } catch (error) {
      if (!(error instanceof ApiError && error.code === "page.title_taken")) {
        throw error;
      }
      const again = await pages.landing(page, target).catch(() => undefined);
      if (again?.node_id === null || again?.node_id === undefined) {
        throw error;
      }
      id = again.node_id;
    }
    reload();
    await go(id);
  }

  const giveBack = (done: boolean) => {
    const container = article.current;
    if (done || asked === undefined || container === null) {
      return;
    }
    const { link, at } = asked;
    focusOn(link.element.isConnected ? link.element : (sameTarget(container, link.target)[at] ?? container));
  };
  const held: HeldDialog = { open, onOpenChange: setOpen, onClosed: giveBack };

  let dialog: ReactNode = null;
  if (asked !== undefined && "landing" in asked) {
    const { landing, link } = asked;
    dialog = (
      <ConfirmDialog
        key={asked.n}
        held={held}
        tone="default"
        title={t("unresolved.createTitle", { title: landing.title })}
        description={placeText(pages, landing.parent_id, t)}
        confirmLabel={t("unresolved.create")}
        sendingLabel={t("unresolved.creating")}
        cancelLabel={t("page.cancel")}
        confirm={() => create(link.target, landing)}
        explain={(error) => (parentGone(error) ? t("unresolved.parentGone") : undefined)}
      />
    );
  } else if (asked !== undefined) {
    dialog = (
      <NoticeDialog
        key={asked.n}
        held={held}
        title={t("unresolved.missingTitle", { target: asked.link.target })}
        description={whyText(asked.why, t)}
      />
    );
  }
  return { unresolved, dialog };
}

/** NoticeDialog says what it says, with one button that closes it; its caller moves the focus then. */
function NoticeDialog({ held, title, description }: { held: HeldDialog; title: string; description: string }) {
  const t = useT();
  return (
    <AlertDialog open={held.open} onOpenChange={held.onOpenChange}>
      <AlertDialogContent
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          held.onClosed(false);
        }}
      >
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
        <div className="flex justify-end">
          <AlertDialogCancel asChild>
            <Button>{t("unresolved.ok")}</Button>
          </AlertDialogCancel>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}

/** sameTarget are the links of container to pages not there whose target is target, in their order. */
function sameTarget(container: HTMLElement | null, target: string): HTMLElement[] {
  return [...(container?.querySelectorAll<HTMLElement>("a.nw-unresolved[data-nw-target]") ?? [])].filter(
    (link) => link.dataset.nwTarget === target
  );
}

/** focusOn gives element the focus, focusable as an anchor's target is when it is not already. */
function focusOn(element: HTMLElement) {
  if (!element.hasAttribute("tabindex") && !(element instanceof HTMLAnchorElement && element.hasAttribute("href"))) {
    element.setAttribute("tabindex", "-1");
  }
  element.focus();
}

/** placeText says where a page made under parent goes: under its path in the tree, or at the notebook's top level. */
function placeText(pages: PageTreeStore, parent: string | null, t: Translate): string {
  if (parent === null) {
    return t("unresolved.atRoot");
  }
  const node = pages.byId(parent);
  if (node === undefined) {
    return t("unresolved.underPage");
  }
  return t("unresolved.under", { place: [...pages.ancestorsOf(parent), node].map((each) => each.name).join(" / ") });
}

/** parentGone tells whether error is the creation's refusal of a parent deleted since the landing was read. */
function parentGone(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.code === "validation_failed" &&
    (error.problem?.errors ?? []).some((each) => each.field === "parent_id")
  );
}

function whyText(why: Why, t: Translate): string {
  switch (why) {
    case "reader":
      return t("unresolved.reader");
    case "embed":
      return t("unresolved.embed");
    case "image":
      return t("unresolved.image");
    case "title_invalid":
      return t("unresolved.titleInvalid");
    case "parent_missing":
      return t("unresolved.parentMissing");
    case "too_deep":
      return t("unresolved.tooDeep");
    case "not_resolvable":
      return t("unresolved.notResolvable");
    case "target_invalid":
      return t("unresolved.targetInvalid");
  }
}
