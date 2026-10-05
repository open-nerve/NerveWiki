import { useState, type ReactNode } from "react";

import { useFocusOnInvalid } from "../components/form-field";
import { useT } from "../i18n/i18n";
import type { FieldError } from "../services/api";
import { formErrors, type FieldMessage, type ProblemTexts } from "./problem-messages";

/** The fields' problems found before sending: a message for each field that has one. */
export type LocalProblems<Field extends string> = Partial<Record<Field, FieldMessage>>;

/**
 * useForm is the sending of a form (M1/P5 design 3.6): nothing goes out
 * while the local check finds a problem; the server's problems show under
 * the fields (a problem code in onField under its field), the rest above
 * the form; the button is disabled while the form is out; after each
 * failure the first invalid field gets the focus; texts says some problem
 * codes the form's way, fieldTexts some field codes of its fields, and
 * explain some problems more than a text does, above the form (M6/P4: the
 * pages a rename's links lock). The form gets ref.
 */
export function useForm<Field extends string>(
  fields: readonly Field[],
  {
    onField = {},
    texts = {},
    fieldTexts,
    explain,
  }: {
    onField?: Readonly<Record<string, Field>>;
    texts?: ProblemTexts;
    fieldTexts?: Readonly<Partial<Record<`${Field}.${FieldError["code"]}`, FieldMessage>>>;
    /** What a problem says above the form, or undefined for its text. */
    explain?: (error: unknown) => ReactNode;
  } = {}
) {
  const t = useT();
  const [local, setLocal] = useState<LocalProblems<Field>>({});
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const [failures, setFailures] = useState(0);
  const ref = useFocusOnInvalid(failures);
  const server = formErrors(failure, t, fields, { onField, texts, fieldTexts });
  const banner: ReactNode = (failure === undefined ? undefined : explain?.(failure)) ?? server.banner;

  /** submit shows found, or runs send when it is empty; it resolves whether send went through. */
  async function submit(found: LocalProblems<Field>, send: () => Promise<void>): Promise<boolean> {
    setLocal(found);
    setFailure(undefined);
    if (Object.keys(found).length > 0) {
      setFailures((n) => n + 1);
      return false;
    }
    setSending(true);
    try {
      await send();
      return true;
    } catch (error) {
      setFailure(error);
      setFailures((n) => n + 1);
      return false;
    } finally {
      setSending(false);
    }
  }

  /** The problem of field: the local check's, else the server's. */
  function problemOf(field: Field): string | undefined {
    const key: FieldMessage | undefined = local[field];
    return key === undefined ? server.fields[field] : t(key);
  }

  return { ref, sending, banner, problemOf, submit };
}
