import { useEffect, useId, useRef, useState, type FormEvent } from "react";

import { useForm } from "../../app/form";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "../../components/ui/dialog";
import { Input } from "../../components/ui/input";
import { Label } from "../../components/ui/label";
import { useT } from "../../i18n/i18n";
import type { ApiTokenCreated } from "../../services/api-token.service";
import { useApiTokens } from "../../stores/context";

/** The lifetimes a new token can have: the server only asks that it end in the future. */
const expiries = [
  { value: "30", label: "tokens.expiry30", days: 30 },
  { value: "90", label: "tokens.expiry90", days: 90 },
  { value: "365", label: "tokens.expiry365", days: 365 },
  { value: "never", label: "tokens.expiryNever", days: undefined },
] as const;

type Expiry = (typeof expiries)[number]["value"];

const dayMs = 86_400_000;

/**
 * CreateTokenDialog creates a personal access token and shows it once
 * (M1/P6 design 3.6). The token lives only in the dialog's content, which
 * the dialog unmounts as it closes: a creation answered after the dialog
 * was cancelled shows its token nowhere, and the list gets the token
 * without it.
 */
export function CreateTokenDialog() {
  const t = useT();
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>{t("tokens.create")}</Button>
      </DialogTrigger>
      {open && <CreateToken close={() => setOpen(false)} />}
    </Dialog>
  );
}

/** CreateToken is the dialog's content while it is open: the form, then the new token, which only Done closes. */
function CreateToken({ close }: { close: () => void }) {
  const [created, setCreated] = useState<ApiTokenCreated>();
  const keepOpen = (event: Event) => {
    if (created !== undefined) {
      event.preventDefault();
    }
  };
  return (
    <DialogContent onEscapeKeyDown={keepOpen} onInteractOutside={keepOpen}>
      {created === undefined ? (
        <CreateTokenForm onCreated={setCreated} cancel={close} />
      ) : (
        <NewToken token={created.token} done={close} />
      )}
    </DialogContent>
  );
}

type Field = "name" | "expires_at" | "current_password";

const fields: readonly Field[] = ["name", "expires_at", "current_password"];

function CreateTokenForm({ onCreated, cancel }: { onCreated: (created: ApiTokenCreated) => void; cancel: () => void }) {
  const apiTokens = useApiTokens();
  const t = useT();
  const expiryId = useId();
  const [name, setName] = useState("");
  const [expiry, setExpiry] = useState<Expiry>("90");
  const [password, setPassword] = useState("");
  const { ref, sending, banner, problemOf, submit } = useForm(fields, {
    "identity.current_password_incorrect": "current_password",
  });
  const expiryProblem = problemOf("expires_at");

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found = {
      ...(name.trim() === "" && { name: "field.required" as const }),
      ...(password === "" && { current_password: "field.required" as const }),
    };
    const days = expiries.find((choice) => choice.value === expiry)?.days;
    void submit(found, async () => {
      onCreated(
        await apiTokens.create({
          name: name.trim(),
          current_password: password,
          ...(days !== undefined && { expires_at: new Date(Date.now() + days * dayMs).toISOString() }),
        })
      );
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      <DialogTitle>{t("tokens.createTitle")}</DialogTitle>
      <DialogDescription>{t("tokens.createBody")}</DialogDescription>
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("tokens.name")}
        name="name"
        autoComplete="off"
        value={name}
        error={problemOf("name")}
        onChange={(event) => setName(event.target.value)}
      />
      <div className="space-y-2">
        <Label htmlFor={expiryId}>{t("tokens.expiry")}</Label>
        <select
          id={expiryId}
          value={expiry}
          aria-invalid={expiryProblem !== undefined || undefined}
          aria-describedby={expiryProblem === undefined ? undefined : `${expiryId}-note`}
          onChange={(event) => setExpiry(event.target.value as Expiry)}
          className="h-9 w-full rounded-md border bg-transparent px-3 text-sm"
        >
          {expiries.map(({ value, label }) => (
            <option key={value} value={value}>
              {t(label)}
            </option>
          ))}
        </select>
        {expiryProblem !== undefined && (
          <p id={`${expiryId}-note`} className="text-sm text-destructive">
            {expiryProblem}
          </p>
        )}
      </div>
      <FormField
        label={t("security.currentPassword")}
        type="password"
        name="current_password"
        autoComplete="current-password"
        value={password}
        error={problemOf("current_password")}
        onChange={(event) => setPassword(event.target.value)}
      />
      <div className="flex justify-end gap-2">
        <Button variant="outline" onClick={cancel}>
          {t("tokens.cancel")}
        </Button>
        <Button type="submit" disabled={sending}>
          {t("tokens.submit")}
        </Button>
      </div>
    </form>
  );
}

/**
 * NewToken shows the token this once, to copy: with the Copy button where
 * the page may use the clipboard (not on plain HTTP at a LAN address),
 * selected on focus everywhere.
 */
function NewToken({ token, done }: { token: string; done: () => void }) {
  const t = useT();
  const id = useId();
  const field = useRef<HTMLInputElement>(null);
  const [copied, setCopied] = useState<boolean>();
  // The token has the focus, selected: copying it is one keystroke, the Copy button or not.
  useEffect(() => field.current?.focus(), []);

  async function copy() {
    // Emptied first, the status says it again on every copy.
    setCopied(undefined);
    try {
      await navigator.clipboard.writeText(token);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="space-y-4">
      <DialogTitle>{t("tokens.createdTitle")}</DialogTitle>
      <DialogDescription>{t("tokens.createdWarning")}</DialogDescription>
      <div className="space-y-2">
        <Label htmlFor={id}>{t("tokens.secret")}</Label>
        <div className="flex gap-2">
          <Input
            ref={field}
            id={id}
            readOnly
            value={token}
            className="font-mono"
            onFocus={(event) => event.currentTarget.select()}
          />
          {"clipboard" in navigator && (
            <Button variant="outline" onClick={() => void copy()}>
              {t("tokens.copy")}
            </Button>
          )}
        </div>
        <output className="block text-sm text-muted-foreground">
          {copied === undefined ? "" : copied ? t("tokens.copied") : t("tokens.copyFailed")}
        </output>
      </div>
      <div className="flex justify-end">
        <Button onClick={done}>{t("tokens.done")}</Button>
      </div>
    </div>
  );
}
