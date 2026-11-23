import { Search } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useId, useState, type FormEvent, type KeyboardEvent, type RefObject } from "react";
import { useMatch, useNavigate, useSearchParams } from "react-router";

import { arrived } from "../../app/arrival";
import type { FieldMessage } from "../../app/problem-messages";
import { composing } from "../../app/shortcuts";
import { useFocusOnInvalid } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { useT } from "../../i18n/i18n";
import { onMac } from "../../lib/platform";
import type { Workspace } from "../../services/workspace.service";
import { useNotebooks } from "../../stores/context";

/** The most bytes of UTF-8 a search's q holds, the server's (M8/P5 design 3.6). */
const maxQueryBytes = 256;

/**
 * queryProblem is the local check of q, trimmed (M8/P5 design 4.2): NUL,
 * then its bytes, alone, as the server checks and says them. The terms,
 * the runs of letters and a q of signs alone the server answers, the
 * results page saying why: its folding is not the browser's, which would
 * count terms otherwise (ß and SS).
 */
function queryProblem(q: string): FieldMessage | undefined {
  if (q.includes("\0")) {
    return "field.q.invalid_format";
  }
  return new TextEncoder().encode(q).length > maxQueryBytes ? "field.q.too_long" : undefined;
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
 * holding NUL it says so under it instead, as an alert: submitted with
 * Enter, the field has the focus already, which gives it none to be heard
 * by (P5B review A-M3). Submitted from another page,
 * the results page's title takes the focus, arrived at; from the results
 * page, it stays in the box. On the results page it holds the address's q
 * as the address changes, unless it was edited since. An Enter that ends
 * an IME's composition submits nothing. Mod+K and Mod+Shift+F give it the
 * focus, its text selected: the shell's, which listens for them
 * (useSearchShortcuts, M13/P2/S3) and gives it field, the box's ref.
 *
 * A pill (M13/P2/S3, decision 785): Search, the magnifier, at its left in
 * a pad of the box's own colour, the one its focus ring is measured on
 * (decision 794), after the field in the DOM, Tab going from the field to
 * it; at its right a hint of Mod+K for the eye, none below md. Where it
 * looks is the field's description, for a screen reader alone.
 */
export const SearchBox = observer(function SearchBox({
  workspace,
  field,
}: {
  workspace: Workspace;
  field: RefObject<HTMLInputElement | null>;
}) {
  const t = useT();
  const navigate = useNavigate();
  const notebooks = useNotebooks(workspace);
  const ids = { field: useId(), hint: useId(), problem: useId() };
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
      <div className="relative">
        <Input
          ref={field}
          id={ids.field}
          type="search"
          enterKeyHint="search"
          autoComplete="off"
          placeholder={t("search.placeholder")}
          value={draft}
          aria-invalid={problem !== undefined || undefined}
          aria-describedby={described}
          aria-keyshortcuts={mac ? "Meta+K Meta+Shift+F" : "Control+K Control+Shift+F"}
          onChange={(event) => {
            setDraft(event.target.value);
            // What was said of the query before goes as it is edited (M5-polish 26.2).
            setProblem(undefined);
          }}
          onKeyDown={keepComposing}
          className="peer h-8 rounded-full border-border bg-surface pr-12 pl-8.75 md:text-compact dark:bg-surface-2"
        />
        {/* The button's pad, the box's colour as the box's focus changes it: its ring is drawn, and measured, on it. */}
        <span className="absolute top-1 left-1.75 grid size-6 rounded-full bg-surface transition-[background-color] duration-150 dark:bg-surface-2 dark:peer-focus-visible:bg-surface">
          <Button
            type="submit"
            variant="ghost"
            size="icon-sm"
            shape="pill"
            aria-label={t("search.submit")}
            className="size-6 text-faint"
          >
            <Search />
          </Button>
        </span>
        <kbd
          aria-hidden
          className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 rounded-[5px] border border-border px-1.25 font-sans text-[11px] leading-4.25 text-faint max-md:hidden"
        >
          {mac ? "⌘K" : "Ctrl K"}
        </kbd>
      </div>
      {problem !== undefined && (
        // A new alert each failed submit: the same problem again is said again.
        <Alert key={failures} id={ids.problem} className="px-2 py-1">
          {t(problem)}
        </Alert>
      )}
      <p id={ids.hint} className="sr-only">
        {scope === undefined ? t("search.inWorkspace") : t("search.inNotebook", { notebook: scope.name })}
      </p>
    </form>
  );
});
