import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState, type MouseEvent, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router";

import { useMounted } from "../../app/mounted";
import { errorText } from "../../app/problem-messages";
import { useT } from "../../i18n/i18n";
import { ApiError } from "../../services/api";
import type { TreeNode } from "../../services/page.service";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { openOf } from "../open-node";
import { useWorkspace } from "../workspace/workspace-layout";

export type NodeTarget = { node: TreeNode; path: string };
export type DuplicateContext = {
  kind?: "sync" | "background" | "import";
  path?: string;
  history?: boolean;
  backgroundUndo?: boolean;
};
type Target = { id: string } | { path: string };
type SourceProblem = { code: string; duplicate_of?: string | null; to?: string | null; path?: string };

const attempted = new WeakMap<PageTreeStore, WeakMap<object, Set<string>>>();
const reading = new WeakMap<PageTreeStore, Promise<void>>();

/** Canonical notebook paths include all ancestors and the final page's .md suffix. */
export function nodeTarget(pages: PageTreeStore, id: string): NodeTarget | undefined {
  const node = pages.nodeById(id);
  const ancestors = pages.nodeAncestors(id);
  if (
    node === undefined ||
    ancestors === undefined ||
    [node, ...ancestors].some((each) => each.notebook_id !== pages.notebookId)
  )
    return undefined;
  return {
    node,
    path: [...ancestors.map((each) => each.name), node.name].join("/") + (node.kind === "page" ? ".md" : ""),
  };
}

export function sourceAtPath(pages: PageTreeStore, path: string): NodeTarget | undefined {
  const target = atPath(pages, path);
  return target !== undefined && pages.isSource(target.node.id) ? target : undefined;
}

function atPath(pages: PageTreeStore, path: string): NodeTarget | undefined {
  for (const node of pages.nodes ?? []) {
    const target = nodeTarget(pages, node.id);
    if (target?.path === path) return target;
  }
  return undefined;
}

/** A node is looked up again at navigation time, including an attachment's current parent. */
export function useNodeNavigate(pages: PageTreeStore): (id: string) => void {
  const navigate = useNavigate();
  const { slug } = useWorkspace();
  const mounted = useMounted();
  return (id) => {
    const target = nodeTarget(pages, id);
    if (!mounted() || pages.notebook === undefined || target === undefined) return;
    const open = openOf(
      { id, kind: target.node.kind, parent: target.node.parent_id },
      `/${slug}/notebooks/${pages.notebookId}`
    );
    void navigate(open.to, { state: open.state });
  };
}

/** UI-only rich text. The app's ordinary errorText keeps its string contract. */
export function duplicateText(
  problem: unknown,
  pages: PageTreeStore,
  navigate: (id: string) => void,
  context: DuplicateContext = {}
): ReactNode | undefined {
  const source = sourceProblem(problem);
  if (source === undefined || (source.code !== "page.source_duplicate" && source.code !== "duplicate_source"))
    return undefined;
  return <Duplicate problem={problem as object} source={source} pages={pages} navigate={navigate} context={context} />;
}

/** History refusals explain the sequence a reader can actually perform, including for admins. */
export function sourceHistoryText(
  problem: unknown,
  pages: PageTreeStore,
  navigate: (id: string) => void,
  context: DuplicateContext = {}
): ReactNode | undefined {
  const source = sourceProblem(problem);
  if (source === undefined || !source.code.startsWith("page.source_")) return undefined;
  return (
    duplicateText(problem, pages, navigate, { ...context, history: true }) ?? (
      <HistoryRefusal problem={problem} context={context} />
    )
  );
}

function sourceProblem(problem: unknown): SourceProblem | undefined {
  const value = problem instanceof ApiError ? problem.problem : problem;
  return typeof value === "object" && value !== null && "code" in value && typeof value.code === "string"
    ? (value as SourceProblem)
    : undefined;
}

function refreshOnce(pages: PageTreeStore, owner: object, target: Target): Promise<void> {
  let owners = attempted.get(pages);
  if (owners === undefined) {
    owners = new WeakMap();
    attempted.set(pages, owners);
  }
  let keys = owners.get(owner);
  if (keys === undefined) {
    keys = new Set();
    owners.set(owner, keys);
  }
  const key = "id" in target ? `id:${target.id}` : `path:${target.path}`;
  if (keys.has(key)) return reading.get(pages) ?? Promise.resolve();
  keys.add(key);
  let pending = reading.get(pages);
  if (pending === undefined) {
    pending = pages
      .load()
      .then(
        () => undefined,
        () => undefined
      )
      .finally(() => {
        if (reading.get(pages) === pending) reading.delete(pages);
      });
    reading.set(pages, pending);
  }
  return pending;
}

function currentTarget(pages: PageTreeStore, target: Target, source: boolean): NodeTarget | undefined {
  const found = "id" in target ? nodeTarget(pages, target.id) : atPath(pages, target.path);
  return pages.notebook !== undefined && found !== undefined && (!source || pages.isSource(found.node.id))
    ? found
    : undefined;
}

function needsRead(pages: PageTreeStore, target: Target): boolean {
  return (
    pages.nodes !== undefined && pages.notebook?.mode === "llm_wiki" && currentTarget(pages, target, true) === undefined
  );
}

const Duplicate = observer(function Duplicate({
  problem,
  source,
  pages,
  navigate,
  context,
}: {
  problem: object;
  source: SourceProblem;
  pages: PageTreeStore;
  navigate: (id: string) => void;
  context: DuplicateContext;
}) {
  const t = useT();
  const imported = source.code === "duplicate_source" || context.kind === "import";
  const target: Target | undefined =
    source.code === "duplicate_source"
      ? typeof source.to === "string"
        ? { path: source.to }
        : undefined
      : typeof source.duplicate_of === "string"
        ? { id: source.duplicate_of }
        : undefined;
  const found = target === undefined ? undefined : currentTarget(pages, target, true);
  const missing = target !== undefined && needsRead(pages, target);
  const id = target !== undefined && "id" in target ? target.id : undefined;
  const path = target !== undefined && "path" in target ? target.path : undefined;
  useEffect(() => {
    const lookup = id !== undefined ? { id } : path !== undefined ? { path } : undefined;
    if (missing && lookup !== undefined) void refreshOnce(pages, problem, lookup);
  }, [pages, problem, missing, id, path]);
  return (
    <span className="space-y-2">
      {imported && (
        <span className="block">{t("source.duplicateImportSkipped", { path: context.path ?? source.path ?? "" })}</span>
      )}
      {target === undefined ? (
        !imported && <span className="block">{t("source.duplicateWithin")}</span>
      ) : found === undefined ? (
        <span className="block">{t("source.duplicateUnavailable")}</span>
      ) : (
        <span className="block">
          {t("source.duplicateAt")}{" "}
          <TargetLink pages={pages} target={target} source owner={problem} navigate={navigate} />
        </span>
      )}
      {!imported && (
        <span className="block">
          {t(context.kind === "background" ? "source.duplicateBatchNotWritten" : "source.duplicateNotWritten")}
        </span>
      )}
      {context.history && (
        <HistoryAdvice background={context.kind === "background" && context.backgroundUndo !== false} />
      )}
    </span>
  );
});

/** A report's refused node can be any visible node, not necessarily the duplicate source. */
export const NodePathLink = observer(function NodePathLink({
  pages,
  id,
  navigate,
}: {
  pages: PageTreeStore;
  id: string;
  navigate: (id: string) => void;
}) {
  return <TargetLink pages={pages} target={{ id }} source={false} navigate={navigate} />;
});

const TargetLink = observer(function TargetLink({
  pages,
  target,
  source,
  owner,
  navigate,
}: {
  pages: PageTreeStore;
  target: Target;
  source: boolean;
  owner?: object;
  navigate: (id: string) => void;
}) {
  const t = useT();
  const { slug } = useWorkspace();
  const location = useLocation();
  const where = useRef(location.key);
  const mounted = useMounted();
  const [, update] = useState(0);
  useEffect(() => {
    where.current = location.key;
  }, [location.key]);
  const found = currentTarget(pages, target, source);
  if (found === undefined) return <span>{t(source ? "source.duplicateUnavailable" : "transfer.refusedSome")}</span>;
  const home = `/${slug}/notebooks/${pages.notebookId}`;
  const open = openOf({ id: found.node.id, kind: found.node.kind, parent: found.node.parent_id }, home);

  async function activate(event: MouseEvent<HTMLAnchorElement>): Promise<void> {
    const current = currentTarget(pages, target, source);
    const modified = event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0;
    if (modified && current !== undefined) {
      const next = openOf({ id: current.node.id, kind: current.node.kind, parent: current.node.parent_id }, home);
      if (next.to === event.currentTarget.getAttribute("href")) return;
    }
    event.preventDefault();
    if (modified) return;
    const from = where.current;
    if (current === undefined && owner !== undefined && needsRead(pages, target))
      await refreshOnce(pages, owner, target);
    if (!mounted() || where.current !== from) return;
    const fresh = currentTarget(pages, target, source);
    if (fresh === undefined) update((n) => n + 1);
    else navigate(fresh.node.id);
  }

  return (
    <a href={open.to} onClick={(event) => void activate(event)} className="underline underline-offset-4">
      {found.path}
    </a>
  );
});

function HistoryAdvice({ background }: { background: boolean }) {
  const t = useT();
  return (
    <>
      {background && <span className="block">{t("source.historyBackground")}</span>}
      <span className="block">{t("source.historyRestore")}</span>
    </>
  );
}

function HistoryRefusal({ problem, context }: { problem: unknown; context: DuplicateContext }) {
  const t = useT();
  const source = sourceProblem(problem);
  const reason = problem instanceof ApiError ? problem : new ApiError(409, source);
  return (
    <span className="space-y-2">
      <span className="block">{errorText(reason, t)}</span>
      <span className="block">
        {t(context.kind === "background" ? "source.duplicateBatchNotWritten" : "source.duplicateNotWritten")}
      </span>
      <HistoryAdvice background={context.kind === "background" && context.backgroundUndo !== false} />
    </span>
  );
}
