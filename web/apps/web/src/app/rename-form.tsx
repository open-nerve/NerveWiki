import { useRef, useState, type FormEvent } from "react";

import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import type { FieldError } from "../services/api";
import { useForm } from "./form";
import type { FieldMessage } from "./problem-messages";

type RenameFormProps = {
  /** The name as the list has it now, a rename made elsewhere too. */
  current: string;
  label: string;
  hint?: string;
  autoComplete?: string;
  /** The local check of the name typed: its problem, or none. */
  check: (name: string) => FieldMessage | undefined;
  /** The field's problems said the form's way. */
  fieldTexts?: Readonly<Partial<Record<`name.${FieldError["code"]}`, FieldMessage>>>;
  /** Sends name, trimmed and other than current; a refusal shows in the form. */
  rename: (name: string) => Promise<unknown>;
  saveLabel: string;
  savedLabel: string;
};

/**
 * RenameForm renames a workspace or a notebook (M2/P5 design 3.6, M3/P4
 * design 3.4; one form, M3 handoff 3): the name goes out only when it
 * changed, trimmed, as the server keeps it. Until it is edited, and again
 * once a save went through, the field shows the current name, a rename by
 * another tab or admin too: Save then sends nothing back over it. A name
 * edited while its rename was out keeps the edit, as DisplayNameForm has
 * it: what it shows then is not what was saved, and the next save sends
 * it.
 */
export function RenameForm({
  current,
  label,
  hint,
  autoComplete,
  check,
  fieldTexts,
  rename,
  saveLabel,
  savedLabel,
}: RenameFormProps) {
  /** What was typed since the last save; none, the current name. */
  const [draft, setDraft] = useState<string>();
  const name = draft ?? current;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, problemOf, submit } = useForm(["name"], { fieldTexts });
  /** How many times the field was edited: a save tells whether the name it sent is still the one shown. */
  const edits = useRef(0);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const problem = check(name);
    const trimmed = name.trim();
    const edit = edits.current;
    setSaved(false);
    void submit(problem === undefined ? {} : { name: problem }, async () => {
      if (trimmed !== current) {
        await rename(trimmed);
      }
      if (edits.current === edit) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={label}
        name="name"
        autoComplete={autoComplete}
        value={name}
        error={problemOf("name")}
        hint={hint}
        onChange={(event) => {
          edits.current++;
          setDraft(event.target.value);
          setSaved(false);
        }}
      />
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {saveLabel}
        </Button>
        <output className="text-sm text-muted-foreground">{saved ? savedLabel : ""}</output>
      </div>
    </form>
  );
}
