import { useRef, useState, type ReactElement } from "react";

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
import { errorText } from "./problem-messages";

type ConfirmDialogProps = {
  /** The button that opens the dialog. */
  trigger: ReactElement;
  title: string;
  description: string;
  confirmLabel: string;
  /** The confirm button's label while confirm is out. */
  sendingLabel: string;
  cancelLabel: string;
  /** What confirming does; when it throws, the dialog stays open with the reason. */
  confirm: () => Promise<void>;
  /**
   * Where the focus goes once confirm went through and took the trigger
   * with it (a revoked token's row); without it, back to the trigger.
   */
  focusAfter?: () => void;
  /**
   * What the user types before confirming, such as the slug of the
   * workspace to delete: label asks for it, and confirm is disabled until
   * the field holds value.
   */
  typedConfirmation?: { label: string; value: string };
};

/**
 * ConfirmDialog asks to confirm what cannot be undone (M1/P6 design 3.5,
 * 3.6): confirm goes out once, however often it is pressed, and nothing
 * closes the dialog while it is out; a refusal stays in the dialog. With
 * typedConfirmation, the user types a word first; closing the dialog
 * clears it.
 */
export function ConfirmDialog({
  trigger,
  title,
  description,
  confirmLabel,
  sendingLabel,
  cancelLabel,
  confirm,
  focusAfter,
  typedConfirmation,
}: ConfirmDialogProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const ready = typedConfirmation === undefined || typed === typedConfirmation.value;
  const [failure, setFailure] = useState<unknown>();
  const [sending, setSending] = useState(false);
  const confirmed = useRef(false);
  const failed = failure === undefined ? undefined : errorText(failure, t);

  async function run() {
    setSending(true);
    setFailure(undefined);
    try {
      await confirm();
    } catch (error) {
      setFailure(error);
      setSending(false);
      return;
    }
    confirmed.current = true;
    setOpen(false);
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!sending) {
          setOpen(next);
          setFailure(undefined);
          setTyped("");
        }
      }}
    >
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent
        onCloseAutoFocus={(event) => {
          if (confirmed.current && focusAfter !== undefined) {
            event.preventDefault();
            focusAfter();
          }
        }}
      >
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
        {typedConfirmation !== undefined && (
          <FormField
            label={typedConfirmation.label}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
          />
        )}
        {failed !== undefined && <Alert>{failed}</Alert>}
        <div className="flex justify-end gap-2">
          <AlertDialogCancel asChild>
            <Button variant="outline" disabled={sending}>
              {cancelLabel}
            </Button>
          </AlertDialogCancel>
          <Button variant="destructive" disabled={sending || !ready} onClick={() => void run()}>
            {sending ? sendingLabel : confirmLabel}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
