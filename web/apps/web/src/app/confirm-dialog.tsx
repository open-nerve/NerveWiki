import { useRef, useState, type ReactElement, type ReactNode } from "react";

import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "../components/ui/alert-dialog";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { focusLost } from "../lib/focus-leaving";
import type { HeldDialog } from "./held-dialog";
import { errorText, type ProblemTexts } from "./problem-messages";

/**
 * How the dialog opens: by its trigger, the focus going back to it, or to
 * focusAfter once confirm went through and took the trigger with it (a
 * revoked token's row); or held by its caller.
 */
type Opening =
  | { trigger: ReactElement; focusAfter?: () => void; held?: never }
  | { held: HeldDialog; trigger?: never; focusAfter?: never };

type ConfirmDialogProps = Opening & {
  title: string;
  description: string;
  /**
   * What more the dialog says below its description, such as what an
   * upgrade's start makes of a page (M11/P1): it describes the dialog with
   * the description.
   */
  more?: ReactNode;
  confirmLabel: string;
  /** The confirm button's label while confirm is out. */
  sendingLabel: string;
  cancelLabel: string;
  /** What confirming does; when it throws, the dialog stays open with the reason. */
  confirm: () => Promise<void>;
  /**
   * What the user types before confirming, such as the slug of the
   * workspace to delete: label asks for it, and confirm is disabled until
   * the field holds value.
   */
  typedConfirmation?: { label: string; value: string };
  /** The dialog's own texts for some problem codes of a refusal. */
  texts?: ProblemTexts;
  /** The dialog's own text of a refusal, where it has one: one that names more than its code does, a list of them. */
  explain?: (error: unknown) => ReactNode | undefined;
  /** How the confirm button looks: destructive, by default, or plain for what is not, a page's creation. */
  tone?: "destructive" | "default";
};

/**
 * ConfirmDialog asks to confirm what cannot be undone (M1/P6 design 3.5,
 * 3.6), or, in the default tone, what is worth a question all the same (a
 * page created for a link, M6/P6 design 7): confirm goes out once, however
 * often it is pressed, and nothing closes the dialog while it is out; a
 * refusal stays in the dialog. With typedConfirmation, the user types a
 * word first, in the field the dialog opens on, where Enter confirms;
 * closing the dialog clears it. While confirm is out its buttons are
 * aria-disabled, not disabled: a disabled button drops the focus. After a
 * refusal the focus is on confirm still (given back to it, lost), and the
 * dialog's alert says why (v0.1 design 13.2, item 12; M12/P2/S1).
 */
export function ConfirmDialog({
  trigger,
  held,
  title,
  description,
  more,
  confirmLabel,
  sendingLabel,
  cancelLabel,
  confirm,
  focusAfter,
  typedConfirmation,
  texts,
  explain,
  tone = "destructive",
}: ConfirmDialogProps) {
  const t = useT();
  const [own, setOwn] = useState(false);
  const open = held?.open ?? own;
  const [typed, setTyped] = useState("");
  const ready = typedConfirmation === undefined || typed === typedConfirmation.value;
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  // Out, whatever was rendered last: a second press, Esc, Cancel before sending shows do nothing.
  const busy = useRef(false);
  const confirmButton = useRef<HTMLButtonElement>(null);
  const confirmed = useRef(false);
  const typedField = useRef<HTMLInputElement>(null);
  const failed = failure === undefined ? undefined : (explain?.(failure) ?? errorText(failure, t, texts));

  async function run() {
    if (busy.current || !ready) {
      return;
    }
    busy.current = true;
    setSending(true);
    setFailure(undefined);
    try {
      await confirm();
    } catch (error) {
      setFailure(error);
      setSending(false);
      if (focusLost()) {
        confirmButton.current?.focus();
      }
      return;
    } finally {
      busy.current = false;
    }
    confirmed.current = true;
    setOpen(false);
  }

  // Sending ends as the dialog closes: a trigger that stays (an export's) opens it anew.
  function setOpen(next: boolean) {
    if (held === undefined) {
      setOwn(next);
    } else {
      held.onOpenChange(next);
    }
    setSending(false);
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!busy.current) {
          setOpen(next);
          setFailure(undefined);
          setTyped("");
        }
      }}
    >
      {trigger !== undefined && <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>}
      <AlertDialogContent
        onOpenAutoFocus={(event) => {
          if (typedConfirmation !== undefined) {
            event.preventDefault();
            typedField.current?.focus();
          }
        }}
        onCloseAutoFocus={(event) => {
          if (held !== undefined) {
            event.preventDefault();
            held.onClosed(confirmed.current);
          } else if (confirmed.current && focusAfter !== undefined) {
            event.preventDefault();
            focusAfter();
          }
          confirmed.current = false;
        }}
      >
        <AlertDialogTitle>{title}</AlertDialogTitle>
        {more === undefined ? (
          <AlertDialogDescription>{description}</AlertDialogDescription>
        ) : (
          // What more it says describes the dialog too, read with its name.
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p>{description}</p>
              <div className="space-y-2 text-foreground">{more}</div>
            </div>
          </AlertDialogDescription>
        )}
        {typedConfirmation !== undefined && (
          <FormField
            ref={typedField}
            label={typedConfirmation.label}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                void run();
              }
            }}
          />
        )}
        {failed !== undefined && <Alert>{failed}</Alert>}
        <div className="flex justify-end gap-2">
          {/* Not disabled while confirm is out: a disabled button drops the focus. Without the typed word there is
              no focus to keep. */}
          <AlertDialogCancel asChild>
            <Button variant="outline" aria-disabled={sending || undefined}>
              {cancelLabel}
            </Button>
          </AlertDialogCancel>
          <Button
            ref={confirmButton}
            variant={tone}
            disabled={!ready}
            aria-busy={sending || undefined}
            aria-disabled={sending || undefined}
            onClick={() => void run()}
          >
            {sending ? sendingLabel : confirmLabel}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
