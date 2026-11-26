import { useId, useRef, useState, type FormEvent, type ReactNode } from "react";

import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { cn } from "../lib/cn";
import { useFocusLeaving } from "../lib/focus-leaving";
import type { FieldError } from "../services/api";
import { useForm, type LocalProblems } from "./form";
import type { FieldMessage } from "./problem-messages";
import { composing } from "./shortcuts";
import {
  rowClass,
  rowControlClass,
  rowDescriptionClass,
  rowProblemClass,
  rowTextClass,
  rowTitleClass,
  SavedOutput,
} from "./settings-page";

type NameRowProps<Field extends string> = {
  /** The name as the account or the list has it now, changed elsewhere too. */
  current: string;
  /** The field, as the server's problems name it. */
  field: Field;
  label: string;
  description?: string;
  autoComplete?: string;
  /** The local check of the name typed: its problem, or none. */
  check: (name: string) => FieldMessage | undefined;
  fieldTexts?: Readonly<Partial<Record<`${Field}.${FieldError["code"]}`, FieldMessage>>>;
  /** Sends name, trimmed and other than current; a refusal shows in the row. */
  save: (name: string) => Promise<unknown>;
  explain?: (error: unknown) => ReactNode;
  saveLabel: string;
  savedLabel: string;
};

/**
 * NameRow is a name changed in a settings card's row (00-M13 4.4.3; M13/P3/S1, decision 822): the display name, a
 * workspace's, a notebook's. Save shows once the field holds other than the name (Enter saves too; leaving the field
 * saves nothing), in a place kept for it after the field, which does not move as it comes and goes; it goes once the
 * name is saved, the field then taking the focus it held (useFocusLeaving). While the name is out, Save says so and
 * does nothing (aria-disabled, aria-busy: disabled, it would drop the focus; v0.1 design 13.2 item 12). Until it is
 * edited, and again once a save went through, the field shows the current name, a rename elsewhere too; a name edited
 * while its save was out keeps the edit, as RenameForm has it. A name other than the current only in its blanks is
 * saved without a request. The field's problem shows under it, its description otherwise describes it: one id, as
 * e2e's noteOf reads it; a refusal of another kind shows under the row. Both say what was wrong with a draft: with
 * the name typed back as it is, they go too (review A's M1). Escape takes a draft back to the current name, Save and
 * what was said of it going too, the focus staying in the field, as a select's takes back the option the keys stepped
 * to (00-M13 4.4.3; M13 closeout, decision 965); the key goes no further up the page then. With no draft, or while a
 * composition goes on, the key is not the row's. Leaving the page drops a draft, as before.
 */
export function NameRow<Field extends string>({
  current,
  field,
  label,
  description,
  autoComplete,
  check,
  fieldTexts,
  save,
  explain,
  saveLabel,
  savedLabel,
}: NameRowProps<Field>) {
  const id = useId();
  /** What was typed since the last save; none, the current name. */
  const [draft, setDraft] = useState<string>();
  const name = draft ?? current;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, problemOf, submit } = useForm([field], { fieldTexts, explain });
  /** How many times the field was edited: a save tells whether the name it sent is still the one shown. */
  const edits = useRef(0);
  const input = useRef<HTMLInputElement>(null);
  const changed = draft !== undefined && draft !== current;
  // A problem and a refusal say what was wrong with a draft: with the name typed back as it is, gone too.
  const problem = changed ? problemOf(field) : undefined;
  const described =
    problem !== undefined ? `${id}-problem` : description !== undefined ? `${id}-description` : undefined;

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !changed) {
      return;
    }
    const found = check(name);
    const trimmed = name.trim();
    const edit = edits.current;
    const problems: LocalProblems<Field> = {};
    if (found !== undefined) {
      problems[field] = found;
    }
    setSaved(false);
    void submit(problems, async () => {
      if (trimmed !== current) {
        await save(trimmed);
      }
      if (edits.current === edit) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit}>
      <div className={rowClass}>
        <div className={rowTextClass}>
          <label htmlFor={id} className={rowTitleClass}>
            {label}
          </label>
          {description !== undefined && (
            <p id={`${id}-description`} className={rowDescriptionClass}>
              {description}
            </p>
          )}
        </div>
        <div className={rowControlClass}>
          <SavedOutput saved={saved} label={savedLabel} />
          <Input
            ref={input}
            id={id}
            name={field}
            autoComplete={autoComplete}
            value={name}
            aria-invalid={problem !== undefined || undefined}
            aria-describedby={described}
            className="h-8 md:w-60 md:text-row"
            onChange={(event) => {
              edits.current++;
              setDraft(event.target.value);
              setSaved(false);
            }}
            onKeyDown={(event) => {
              if (event.key !== "Escape" || draft === undefined || composing(event.nativeEvent)) {
                return;
              }
              event.preventDefault();
              event.stopPropagation();
              setDraft(undefined);
            }}
          />
          {/* Save's place, kept: the field does not move as it comes and goes (review B's M6). */}
          <div className="flex max-md:order-2 max-md:empty:hidden md:min-w-15 md:justify-end">
            {changed && <SaveButton label={saveLabel} busy={sending} onLeave={() => input.current?.focus()} />}
          </div>
          {problem !== undefined && (
            <p id={`${id}-problem`} className={cn(rowProblemClass, "max-md:order-1 md:-me-21 md:ps-19")}>
              {problem}
            </p>
          )}
        </div>
      </div>
      {changed && banner !== undefined && (
        <div className="px-4.5 pb-3.5">
          <Alert>{banner}</Alert>
        </div>
      )}
    </form>
  );
}

/**
 * SaveButton is a NameRow's Save, mounted while the field holds a change: going with the focus, it gives it back. A
 * component of its own: useFocusLeaving takes its element as it mounts, which NameRow's mount would find none of.
 */
function SaveButton({ label, busy, onLeave }: { label: string; busy: boolean; onLeave: () => void }) {
  const button = useRef<HTMLButtonElement>(null);
  useFocusLeaving(button, onLeave);
  return (
    <Button
      ref={button}
      type="submit"
      size="sm"
      shape="pill"
      aria-disabled={busy || undefined}
      aria-busy={busy || undefined}
    >
      {label}
    </Button>
  );
}
