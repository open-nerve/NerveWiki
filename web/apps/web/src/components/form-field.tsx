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
  const input = (
    <Input
      id={id}
      type={type === "password" && shown ? "text" : type}
      aria-invalid={error !== undefined || undefined}
      aria-describedby={note === undefined ? undefined : `${id}-note`}
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
    </div>
  );
}

/**
 * useFocusOnInvalid moves the focus to the first invalid field of the form
 * after each failed submit (failures counts them): a screen reader then
 * reads the field with its problem, which appeared silently under it. The
 * form gets the ref returned.
 */
export function useFocusOnInvalid(failures: number): RefObject<HTMLFormElement | null> {
  const form = useRef<HTMLFormElement>(null);
  useEffect(() => {
    if (failures > 0) {
      form.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
    }
  }, [failures]);
  return form;
}
