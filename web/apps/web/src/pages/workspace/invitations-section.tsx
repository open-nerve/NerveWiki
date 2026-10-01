import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useForm } from "../../app/form";
import { invitationLink } from "../../app/invitation-link";
import { NotLoaded } from "../../app/not-loaded";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { Label } from "../../components/ui/label";
import { NativeSelect } from "../../components/ui/native-select";
import { formatDate } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import type { WorkspaceInvitation } from "../../services/invitation.service";
import type { WorkspaceRole } from "../../services/member.service";
import type { Workspace } from "../../services/workspace.service";
import { useInvitations, useStore } from "../../stores/context";
import { RoleOptions } from "./role-options";

/**
 * InvitationsSection is the workspace's pending invitations, an admin's
 * (M2/P6 design 3.3): one invites an address, then copies the link and
 * sends it; the server sends no mail.
 */
export const InvitationsSection = observer(function InvitationsSection({ workspace }: { workspace: Workspace }) {
  const invitations = useInvitations(workspace);
  const t = useT();
  const { error, mutate } = useSWR(["invitations", workspace.id], () => invitations.load());
  const [copied, setCopied] = useState<string>();
  // A withdrawn invitation's row leaves with its button: the focus comes to the section's heading instead.
  const heading = useRef<HTMLHeadingElement>(null);
  const list = invitations.list;

  let rows;
  if (list === undefined) {
    rows = <NotLoaded error={error} retry={() => void mutate()} />;
  } else if (list.length === 0) {
    rows = <p className="text-muted-foreground">{t("invitations.empty")}</p>;
  } else {
    rows = (
      <ul aria-label={t("invitations.title")} className="divide-y rounded-md border">
        {list.map((invitation) => (
          <InvitationRow
            key={invitation.id}
            workspace={workspace}
            invitation={invitation}
            copied={(done) => setCopied(done ? invitation.email : undefined)}
            withdrawn={() => {
              setCopied(undefined);
              heading.current?.focus();
            }}
          />
        ))}
      </ul>
    );
  }

  return (
    <section className="space-y-4">
      <div className="max-w-xl space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">
          {t("invitations.title")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("invitations.body")}</p>
      </div>
      <InviteForm workspace={workspace} />
      {rows}
      <output className="block text-sm text-muted-foreground">
        {copied === undefined ? "" : t("invitations.copied", { email: copied })}
      </output>
    </section>
  );
});

/** InviteForm invites an address with a role; a member's address, or one invited already, is refused under the field. */
function InviteForm({ workspace }: { workspace: Workspace }) {
  const invitations = useInvitations(workspace);
  const t = useT();
  const roleId = useId();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<WorkspaceRole>("member");
  const [invited, setInvited] = useState<string>();
  const { ref, sending, banner, problemOf, submit } = useForm(["email", "role"]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    // The browser takes the ASCII spaces off an e-mail field's value; trim takes the others too, such as an
    // ideographic space an input method typed.
    const address = email.trim();
    setInvited(undefined);
    void submit(address === "" ? { email: "field.required" } : {}, async () => {
      const sent = await invitations.invite({ email: address, role });
      setEmail("");
      // As the server keeps it: in lower case, as the list shows it.
      setInvited(sent.email);
    });
  }

  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="max-w-xl space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-56 flex-1">
          <FormField
            label={t("form.email")}
            type="email"
            name="email"
            autoComplete="off"
            value={email}
            error={problemOf("email")}
            onChange={(event) => {
              setEmail(event.target.value);
              setInvited(undefined);
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor={roleId}>{t("invitations.role")}</Label>
          <NativeSelect
            id={roleId}
            name="role"
            value={role}
            onChange={(event) => setRole(event.target.value as WorkspaceRole)}
          >
            <RoleOptions />
          </NativeSelect>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={sending}>
          {t("invitations.invite")}
        </Button>
        <output className="text-sm text-muted-foreground">
          {invited === undefined ? "" : t("invitations.invited", { email: invited })}
        </output>
      </div>
    </form>
  );
}

type InvitationRowProps = {
  workspace: Workspace;
  invitation: WorkspaceInvitation;
  /** Called with whether the link went to the clipboard. */
  copied: (done: boolean) => void;
  /** Where the focus goes once the invitation is withdrawn, with the row. */
  withdrawn: () => void;
};

/**
 * InvitationRow is a pending invitation: the address, the role, when it
 * was sent; its link goes to the clipboard, or, where the browser has none
 * for this page (an address served over HTTP but localhost), into a field
 * to copy it from.
 */
const InvitationRow = observer(function InvitationRow({
  workspace,
  invitation,
  copied,
  withdrawn,
}: InvitationRowProps) {
  const invitations = useInvitations(workspace);
  const { preferences } = useStore();
  const t = useT();
  const [shown, setShown] = useState(false);
  const link = invitationLink(window.location.origin, invitation);

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      setShown(false);
      copied(true);
    } catch {
      setShown(true);
      copied(false);
    }
  }

  return (
    <li className="space-y-3 p-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0 space-y-1">
          <p className="font-medium break-all">{invitation.email}</p>
          <p className="text-sm text-muted-foreground">
            {t(`role.${invitation.role}`)} ·{" "}
            {t("invitations.invitedOn", { date: formatDate(invitation.created_at, preferences.locale) })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            aria-label={t("invitations.copyLabel", { email: invitation.email })}
            onClick={() => void copy()}
          >
            {t("invitations.copy")}
          </Button>
          <ConfirmDialog
            trigger={
              <Button variant="outline" aria-label={t("invitations.withdrawLabel", { email: invitation.email })}>
                {t("invitations.withdraw")}
              </Button>
            }
            title={t("invitations.withdrawTitle", { email: invitation.email })}
            description={t("invitations.withdrawBody")}
            confirmLabel={t("invitations.withdraw")}
            sendingLabel={t("invitations.withdrawing")}
            cancelLabel={t("invitations.cancel")}
            confirm={() => invitations.withdraw(invitation.id)}
            focusAfter={withdrawn}
          />
        </div>
      </div>
      {shown && <LinkField email={invitation.email} link={link} />}
    </li>
  );
});

/** LinkField shows link to copy by hand: it takes the focus as it shows, and selects the link with it. */
function LinkField({ email, link }: { email: string; link: string }) {
  const t = useT();
  const field = useRef<HTMLInputElement>(null);
  useEffect(() => field.current?.focus(), []);
  return (
    <FormField
      ref={field}
      label={t("invitations.linkLabel", { email })}
      hint={t("invitations.linkHint")}
      readOnly
      value={link}
      onFocus={(event) => event.target.select()}
    />
  );
}
