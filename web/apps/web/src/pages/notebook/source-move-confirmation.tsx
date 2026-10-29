import { reaction } from "mobx";
import { useCallback, useEffect, useRef, useState } from "react";
import { useLocation } from "react-router";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useMounted } from "../../app/mounted";
import { canMoveNode } from "../../app/source-permissions";
import { useT } from "../../i18n/i18n";
import type { PageTreeStore } from "../../stores/page-tree.store";

type Pending = { valid: () => boolean; answer: (accepted: boolean) => void };
class MoveCancelled extends Error {}

/** All three move entries ask the same question, before sending any write. */
export function useSourceMove(pages: PageTreeStore) {
  const t = useT();
  const mounted = useMounted();
  const location = useLocation();
  const where = useRef(location.key);
  const busy = useRef(false);
  const request = useRef<Pending | undefined>(undefined);
  const focus = useRef<HTMLElement | null>(null);
  const focusAt = useRef<string | undefined>(undefined);
  const [pending, setPending] = useState<Pending>();

  useEffect(() => () => request.current?.answer(false), []);
  useEffect(() => {
    where.current = location.key;
    request.current?.answer(false);
  }, [location.key]);
  useEffect(
    () =>
      pending === undefined
        ? undefined
        : reaction(
            pending.valid,
            (valid) => {
              if (!valid) pending.answer(false);
            },
            { fireImmediately: true }
          ),
    [pending]
  );

  const move = useCallback(
    async (id: string, parent: string | null, send: (beforeSend: () => void) => Promise<void>): Promise<boolean> => {
      if (busy.current || !mounted()) return false;
      const before = moveState(pages, id, parent);
      if (before === undefined) return false;
      const started = location.key;
      const notebook = notebookState(pages);
      const stillHere = () => mounted() && where.current === started && notebookState(pages) === notebook;
      const valid = () => stillHere() && moveState(pages, id, parent) === before;
      if (!valid()) return false;
      busy.current = true;
      try {
        const current = pages.notebook;
        if (
          current?.role === "editor" &&
          current.mode === "llm_wiki" &&
          parent !== null &&
          (parent === current.source_root_id || pages.isSource(parent))
        ) {
          focus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
          focusAt.current = started;
          const accepted = await new Promise<boolean>((resolve) => {
            const asking: Pending = {
              valid,
              answer: (answer) => {
                if (request.current !== asking) return;
                request.current = undefined;
                if (mounted()) setPending(undefined);
                resolve(answer);
              },
            };
            request.current = asking;
            setPending(asking);
          });
          if (!accepted) return false;
        }
        if (!valid()) return false;
        await send(() => {
          if (!valid()) throw new MoveCancelled();
        });
        // The successful move itself changes the node's parent and source capability.
        // Only the initiating route/generation and notebook still govern its feedback.
        return stillHere();
      } catch (error) {
        if (error instanceof MoveCancelled || !stillHere()) return false;
        throw error;
      } finally {
        busy.current = false;
      }
    },
    [pages, mounted, location.key]
  );

  const confirmation = (
    <ConfirmDialog
      held={{
        open: pending !== undefined,
        onOpenChange: (open) => {
          if (!open) pending?.answer(false);
        },
        onClosed: () => {
          if (mounted() && where.current === focusAt.current && focus.current?.isConnected) focus.current.focus();
        },
      }}
      title={t("page.sourceMoveTitle")}
      description={t("page.sourceMoveBody")}
      confirmLabel={t("page.move")}
      sendingLabel={t("page.moving")}
      cancelLabel={t("page.cancel")}
      tone="default"
      confirm={async () => {
        pending?.answer(pending.valid());
      }}
    />
  );
  return { move, confirmation };
}

function notebookState(pages: PageTreeStore): string {
  const notebook = pages.notebook;
  return JSON.stringify([notebook?.mode, notebook?.source_root_id, notebook?.role]);
}

/** Refreshing an unchanged tree keeps a choice; a changed node or ancestry cancels it. */
function moveState(pages: PageTreeStore, id: string, parent: string | null): string | undefined {
  if (!canMoveNode(pages, id, parent)) return undefined;
  const node = pages.nodeById(id);
  const ancestors = pages.nodeAncestors(id);
  const destination = parent === null ? null : pages.nodeById(parent);
  const above = parent === null ? [] : pages.nodeAncestors(parent);
  if (ancestors === undefined || above === undefined || (parent !== null && destination?.kind !== "page")) {
    return undefined;
  }
  return JSON.stringify([notebookState(pages), node, ancestors, destination, above]);
}
