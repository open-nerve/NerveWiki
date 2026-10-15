import { Search } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { useMatch, useNavigate, useSearchParams } from "react-router";

import { arrived } from "../../app/arrival";
import type { FieldMessage } from "../../app/problem-messages";
import { dialogOpen, isModShift, onMac } from "../../app/shortcuts";
import { useFocusOnInvalid } from "../../components/form-field";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { useT } from "../../i18n/i18n";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks } from "../../stores/context";

/** The most bytes of UTF-8 a search's q holds, the server's (M8/P5 design 3.6). */
const maxQueryBytes = 256;

/**
 * queryProblem is the local check of q, trimmed (M8/P5 design 4.2): its
 * bytes and NUL alone, as the server's 422 says them. The terms, the runs
 * of letters and a q of signs alone the server answers, the results page
 * saying why: its folding is not the browser's, which would count terms
 * otherwise (ß and SS).
 */
function queryProblem(q: string): FieldMessage | undefined {
  if (new TextEncoder().encode(q).length > maxQueryBytes) {
    return "field.q.too_long";
  }
  return q.includes("\0") ? "field.q.invalid_format" : undefined;
}

/** composing tells whether a key press ends or goes into an IME's composition: Safari's Enter after it has 229. */
function composing(event: Pick<globalThis.KeyboardEvent, "isComposing" | "keyCode">): boolean {
  return event.isComposing || event.keyCode === 229;
}

/** keepComposing keeps an Enter that ends an IME's composition from submitting the box: it confirms the IME's text. */
function keepComposing(event: KeyboardEvent<HTMLInputElement>): void {
  if (event.key === "Enter" && composing(event.nativeEvent)) {
    event.preventDefault();
  }
}

/**
 * SearchBox is a workspace's search box, at the top of its left column
 * (M8 design 4.13; M8/P5 design 4.2): it goes to the results page of what
 * it holds, trimmed, a step of the history, and sends nothing itself, so
 * it is no useForm's (v0.1 design 13.2, item 12). Where it looks its hint
 * says, which describes the field: in a notebook open, or the one the
 * results page shown searches, else the whole workspace. A q too long or
 * holding NUL it says so under it instead. Submitted from another page,
 * the results page's title takes the focus, arrived at; from the results
 * page, it stays in the box. On the results page it holds the address's q
 * as the address changes, unless it was edited since. An Enter that ends
 * an IME's composition submits nothing. Mod+Shift+F (Cmd on macOS, Ctrl
 * elsewhere) gives it the focus, its text selected; not while a key is
 * held, nor in a composition, whose half-typed text the browser would
 * write into the page as the focus left it (13.2, item 20); over a dialog
 * it only keeps the browser's own use of the keys.
 */
export const SearchBox = observer(function SearchBox({ workspace }: { workspace: Workspace }) {
  const t = useT();
  const navigate = useNavigate();
  const notebooks = useNotebooks(workspace);
  const ids = { field: useId(), hint: useId(), problem: useId() };
  const field = useRef<HTMLInputElement>(null);
  const [mac] = useState(onMac);
  // Not useParams: the column is the workspace's, above the routes that name a notebook.
  const notebookPage = useMatch("/:slug/notebooks/:id/*");
  const resultsPage = useMatch("/:slug/search");
  const [params] = useSearchParams();
  const asked = resultsPage === null ? undefined : (params.get("q") ?? "");
  const scopeId = (resultsPage === null ? notebookPage?.params.id : params.get("notebook_id"))?.toLowerCase();
  const scope = scopeId === undefined ? undefined : notebooks.list?.find((each) => each.id === scopeId);
  // What the box holds, and the q it last took from the address or sent: one it holds still is not edited since.
  const [draft, setDraft] = useState(asked ?? "");
  const [taken, setTaken] = useState(asked);
  if (asked !== undefined && asked !== taken) {
    setTaken(asked);
    if (draft === (taken ?? "")) {
      setDraft(asked);
    }
  }
  const [problem, setProblem] = useState<FieldMessage>();
  const [failures, setFailures] = useState(0);
  const form = useFocusOnInvalid(failures);

  useEffect(() => {
    const onShortcut = (event: globalThis.KeyboardEvent) => {
      if (!isModShift(event, "f", mac) || composing(event)) {
        return;
      }
      event.preventDefault();
      if (!event.repeat && !dialogOpen()) {
        field.current?.focus();
        field.current?.select();
      }
    };
    window.addEventListener("keydown", onShortcut);
    return () => window.removeEventListener("keydown", onShortcut);
  }, [mac]);

  function onSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    const q = draft.trim();
    if (q === "") {
      return;
    }
    const found = queryProblem(q);
    setProblem(found);
    if (found !== undefined) {
      setFailures((n) => n + 1);
      return;
    }
    setDraft(q);
    setTaken(q);
    const query = new URLSearchParams({ q });
    if (scope !== undefined) {
      query.set("notebook_id", scope.id);
    }
    void navigate(`/${workspace.slug}/search?${query.toString()}`, {
      state: resultsPage === null ? arrived : undefined,
    });
  }

  const described = problem === undefined ? ids.hint : `${ids.problem} ${ids.hint}`;
  return (
    <form
      ref={form}
      // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- the form itself is the landmark (M8/P5 design 4.2): <search> would wrap it in another element
      role="search"
      aria-label={t("search.form", { workspace: workspace.name })}
      noValidate
      onSubmit={onSubmit}
      className="space-y-1"
    >
      <Label htmlFor={ids.field} className="sr-only">
        {t("search.field")}
      </Label>
      <div className="flex gap-1">
        <Input
          ref={field}
          id={ids.field}
          type="search"
          enterKeyHint="search"
          autoComplete="off"
          value={draft}
          aria-invalid={problem !== undefined || undefined}
          aria-describedby={described}
          aria-keyshortcuts={mac ? "Meta+Shift+F" : "Control+Shift+F"}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={keepComposing}
        />
        <Button type="submit" variant="outline" size="icon" aria-label={t("search.submit")}>
          <Search />
        </Button>
      </div>
      {problem !== undefined && (
        <p id={ids.problem} className="text-sm text-destructive">
          {t(problem)}
        </p>
      )}
      <p id={ids.hint} className="px-1 text-xs text-muted-foreground">
        {scope === undefined ? t("search.inWorkspace") : t("search.inNotebook", { notebook: scope.name })}
      </p>
    </form>
  );
});
