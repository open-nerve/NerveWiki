import { Eye, EyeOff } from "lucide-react";
import { useId, useState, type ComponentProps } from "react";

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
};

/**
 * FormField is a labelled input with its problem under it. A password field
 * has a button that shows what was typed (M1/P5 design 3.6).
 */
export function FormField({ label, error, hint, type, ...props }: FormFieldProps) {
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
            aria-label={shown ? t("password.hide") : t("password.show")}
            aria-pressed={shown}
            onClick={() => setShown(!shown)}
          >
            {shown ? <EyeOff /> : <Eye />}
          </Button>
        </div>
      ) : (
        input
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
