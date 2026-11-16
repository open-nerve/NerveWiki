import { Eye, EyeOff } from "lucide-react";
import { useEffect, useId, useRef, useState, type ComponentProps, type ReactNode, type RefObject } from "react";

import { useT } from "../i18n/i18n";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";

type FormFieldProps = Omit<ComponentProps<"input">, "id"> & {
  label: string;
  /** The field's problem, shown under it and announced with it. */
  error?: string | undefined;
  /** A hint under the field while it has no problem. */
  hint?: string | undefined;
  /** What shows after the input, which the hint says too: an attachment's extension, which a rename keeps. */
  suffix?: ReactNode;
};

/**
 * FormField is a labelled input with its problem under it. A password field
 * has a button that shows what was typed (M1/P5 design 3.6).
 */
export function FormField({ label, error, hint, suffix, type, ...props }: FormFieldProps) {
  const id = useId();
  const t = useT();
  const [shown, setShown] = useState(false);
  const note = error ?? hint;
  // A suffix's hint stays with a problem: the suffix shows, unread, what it says.
  const kept = suffix !== undefined && error !== undefined ? hint : undefined;
  const described = [note === undefined ? "" : `${id}-note`, kept === undefined ? "" : `${id}-hint`].join(" ").trim();
  const input = (
    <Input
      id={id}
      type={type === "password" && shown ? "text" : type}
      aria-invalid={error !== undefined || undefined}
      aria-describedby={described === "" ? undefined : described}
      {...props}
    />
  );
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      {type === "password" ? (
        <div className="flex gap-1">
          {input}
          <Button
            variant="ghost"
            size="icon"
            aria-label={t("password.show")}
            aria-pressed={shown}
            onClick={() => setShown(!shown)}
          >
            {shown ? <EyeOff /> : <Eye />}
          </Button>
        </div>
      ) : suffix === undefined ? (
        input
      ) : (
        <div className="flex items-center gap-1">
          {input}
          <span aria-hidden className="text-sm text-muted-foreground">
            {suffix}
          </span>
        </div>
      )}
      {note !== undefined && (
        <p
          id={`${id}-note`}
          className={error === undefined ? "text-sm text-muted-foreground" : "text-sm text-destructive"}
        >
          {note}
        </p>
      )}
      {kept !== undefined && (
        <p id={`${id}-hint`} className="text-sm text-muted-foreground">
          {kept}
        </p>
      )}
    </div>
  );
}

/**
 * useFocusOnInvalid moves the focus to the first invalid field of the form
 * after each failed submit (failures counts them): a screen reader then
 * reads the field with its problem, which appeared silently under it.
 * Without one, only the banner above the form saying the problem, the
 * banner's alert takes the focus when it was lost as the failure came
 * (useForm's lost, its data-form-banner): it would stay in the body
 * otherwise (M12/P2/S1). The form gets the ref returned.
 */
export function useFocusOnInvalid(failures: number, lost = false): RefObject<HTMLFormElement | null> {
  const form = useRef<HTMLFormElement>(null);
  useEffect(() => {
    if (failures === 0) {
      return;
    }
    const invalid = form.current?.querySelector<HTMLElement>('[aria-invalid="true"]');
    if (invalid) {
      invalid.focus();
    } else if (lost) {
      const alert = form.current?.querySelector("[data-form-banner]")?.closest<HTMLElement>('[role="alert"]');
      if (alert) {
        // React renders no tabIndex on the Alert: each failure's is a new element, set again.
        alert.tabIndex = -1;
        alert.focus();
      }
    }
  }, [failures, lost]);
  return form;
}
