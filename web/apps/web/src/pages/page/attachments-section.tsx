import { reaction } from "mobx";
import { Upload as UploadIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type RefObject } from "react";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { writesPages } from "../../app/effective-role";
import { useFileDrop } from "../../app/file-drop";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import { embedOf } from "../../lib/asset-kind";
import { cn } from "../../lib/cn";
import type { Asset } from "../../services/asset.service";
import type { Notebook } from "../../services/notebook.service";
import type { AssetList, AssetStore } from "../../stores/asset.store";
import { useAssets, useStore } from "../../stores/context";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { MoveAssetDialog, RenameAssetDialog } from "./asset-dialogs";
import { rereadIn } from "./assets-expiry";
import { AttachmentRow, type AssetAction } from "./attachment-row";
import { UploadRows } from "./upload-rows";

/** The dialogs of the section's rows. */
type Dialog = Exclude<AssetAction, "copy">;

/**
 * AttachmentsSection is the attachments under a page, in its middle column
 * below its subpages, or under the notebook's root, on its home (M7/P4
 * design 3.5): each a row (AttachmentRow), a hundred at a time, More
 * reading the next hundred. A reader sees it only when there are some. A
 * writer uploads files, by Upload or dropped on it (a folder is not: it
 * says to import one), and their uploads show above the list (UploadRows);
 * what began, here or dropped elsewhere, and what was uploaded are said,
 * unseen. The list is read
 * again before the first of its addresses expires: a link followed must be
 * signed still, and one signed on a click would open a tab the browser
 * takes for a pop-up. Copy embed copies a row's embed; where the page has
 * no clipboard, the embed shows in a field to copy from, which takes the
 * focus from the menu as it closes, or as it shows. The section holds
 * the dialogs of its rows, which a row read away (the rename done, another
 * tab's) does not take with it; one closed gives the focus back to its
 * row's menu, the section's title when the row is gone.
 */
export const AttachmentsSection = observer(function AttachmentsSection({
  notebook,
  parent,
  editorsElsewhere = false,
}: {
  notebook: Notebook;
  /** The page whose attachments these are; null for the notebook's root. */
  parent: string | null;
  /** Whether the editor's uploads show elsewhere: by the editor, as the page is edited (M7/P4 design 5.3). */
  editorsElsewhere?: boolean;
}) {
  const t = useT();
  const assets = useAssets(notebook);
  const { instance } = useStore();
  useSWR("instance", () => instance.load());
  const { error, mutate } = useSWR(["assets", notebook.id, parent ?? "root"], () => assets.load(parent));
  const list = assets.listOf(parent);
  useExpiry(list, () => void mutate());
  const writer = writesPages(notebook.role);
  const uploads = assets.uploads.filter(
    (upload) => upload.parent === parent && !(editorsElsewhere && upload.fromEditor)
  );
  const headingId = useId();
  const section = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const uploadButton = useRef<HTMLButtonElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const [folders, setFolders] = useState(false);
  const [copied, setCopied] = useState<{ embed: string; name: string; done: boolean } | undefined>(undefined);
  const copyField = useRef<HTMLInputElement>(null);
  // Whether the menu closing chose Copy embed: the field of an embed not copied takes the focus then.
  const copying = useRef(false);
  const notice = useNotice(assets, parent);
  // The attachment of the dialog last opened, which stays as it closes, and which dialog is open.
  const [target, setTarget] = useState<Asset | undefined>(undefined);
  const [dialog, setDialog] = useState<Dialog | undefined>(undefined);
  const upload = (files: readonly File[]) =>
    assets.upload(parent, files, t("asset.untitled"), { maxBytes: instance.info?.asset_max_bytes });
  const drop = useFileDrop(writer, (dropped) => {
    setFolders(dropped.folders);
    upload(dropped.files);
  });
  // The attachment's link the focus falls to as More goes: held by its row.
  const focused = useRef<HTMLAnchorElement>(null);
  const more = useLoadMore(
    async () => {
      const added = await assets.more(parent);
      const read = assets.listOf(parent);
      return {
        added: added.map((asset) => asset.id),
        whole: read !== undefined && read.next === null,
        last: read?.assets.at(-1)?.id,
      };
    },
    () => focused.current,
    heading
  );

  if (!writer && uploads.length === 0 && (list === undefined ? error === undefined : list.assets.length === 0)) {
    return null;
  }

  function act(action: AssetAction, asset: Asset) {
    setCopied(undefined);
    if (action === "copy") {
      copying.current = true;
      const link = asset.link;
      if (link !== null) {
        const embed = embedOf(link);
        const copy = async () => {
          try {
            await navigator.clipboard.writeText(embed);
          } catch {
            // Without a secure context the page has no clipboard: the copy fails.
            setCopied({ embed, name: asset.name, done: false });
            return;
          }
          setCopied({ embed, name: asset.name, done: true });
          notice.say(t("asset.copied"));
        };
        void copy();
      }
      return;
    }
    setTarget(asset);
    setDialog(action);
  }

  /**
   * menuClosed gives the focus, as a row's menu closes after Copy embed, to
   * the field of the embed not copied, where it failed already (no
   * clipboard: at once); the menu gives it back to its button otherwise.
   */
  function menuClosed(event: Event) {
    const field = copyField.current;
    if (copying.current && field !== null) {
      event.preventDefault();
      field.focus();
    }
    copying.current = false;
  }

  // The attachment as the list has it now: a rename read again renames it.
  const shown = target && (list?.assets.find((asset) => asset.id === target.id) ?? target);

  /** held is which dialog of shown's, which, closed, gives the focus to its row's menu, or the title once it is gone. */
  function held(which: Dialog, asset: Asset) {
    return {
      open: dialog === which,
      onOpenChange: (open: boolean) => setDialog(open ? which : undefined),
      onClosed: () =>
        (section.current?.querySelector<HTMLElement>(`[data-actions-of="${asset.id}"]`) ?? heading.current)?.focus(),
    };
  }

  return (
    <section
      ref={section}
      aria-labelledby={headingId}
      className={cn("space-y-2 rounded-md", drop.over && "ring-2 ring-primary ring-offset-4")}
      {...drop.handlers}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2
          id={headingId}
          ref={heading}
          tabIndex={-1}
          className="text-sm font-medium text-muted-foreground outline-none"
        >
          {t("asset.title")}
        </h2>
        {writer && (
          <>
            <input
              ref={picker}
              type="file"
              multiple
              hidden
              onChange={(event) => {
                const files = [...(event.target.files ?? [])];
                // The same file chosen again is a change too.
                event.target.value = "";
                setFolders(false);
                upload(files);
              }}
            />
            <Button ref={uploadButton} variant="outline" onClick={() => picker.current?.click()}>
              <UploadIcon />
              {t("asset.upload")}
            </Button>
          </>
        )}
      </div>
      {/* Said as it changes: what began, what was uploaded, an embed copied. */}
      <p aria-live="polite" className="sr-only">
        {notice.text}
      </p>
      {writer && list !== undefined && list.assets.length === 0 && uploads.length === 0 && (
        <p className="text-sm text-muted-foreground">{t("asset.dropHint")}</p>
      )}
      {folders && <Alert>{t("asset.folders")}</Alert>}
      {copied !== undefined &&
        (copied.done ? (
          <p className="text-sm text-muted-foreground">{t("asset.copied")}</p>
        ) : (
          <CopyField key={copied.embed} embed={copied.embed} name={copied.name} field={copyField} />
        ))}
      {uploads.length > 0 && (
        <UploadRows assets={assets} uploads={uploads} left={() => (uploadButton.current ?? heading.current)?.focus()} />
      )}
      {list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        list.assets.length > 0 && (
          <ul aria-labelledby={headingId} className="divide-y rounded-md border">
            {list.assets.map((asset) => (
              <AttachmentRow
                key={asset.id}
                asset={asset}
                writer={writer}
                act={act}
                menuClosed={menuClosed}
                linkRef={asset.id === more.focusing ? focused : undefined}
              />
            ))}
          </ul>
        )
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {list?.next != null && <LoadMoreButton more={more} label={t("asset.more")} />}
      {shown !== undefined && writer && (
        <>
          <RenameAssetDialog notebook={notebook} asset={shown} held={held("rename", shown)} />
          <MoveAssetDialog notebook={notebook} asset={shown} held={held("move", shown)} />
          <ConfirmDialog
            held={held("delete", shown)}
            title={t("asset.deleteTitle", { name: shown.name })}
            description={t("asset.deleteBody")}
            confirmLabel={t("page.delete")}
            sendingLabel={t("page.deleting")}
            cancelLabel={t("page.cancel")}
            confirm={() => assets.remove(shown.id, shown.parent_id)}
          />
        </>
      )}
    </section>
  );
});

/**
 * CopyField is an embed the page could not copy, the clipboard out of its
 * reach (a page served without HTTPS): in a field, selected as it is
 * focused, which it is as it shows, to copy by hand.
 */
function CopyField({ embed, name, field }: { embed: string; name: string; field: RefObject<HTMLInputElement | null> }) {
  const t = useT();
  // A copy that failed once its menu closed; one that failed at once, the menu takes it back from, and gives it.
  useEffect(() => {
    field.current?.focus();
  }, [field]);
  return (
    <FormField
      ref={field}
      label={t("asset.embedOf", { name })}
      hint={t("asset.copyFailed")}
      readOnly
      value={embed}
      onFocus={(event) => event.currentTarget.select()}
    />
  );
}

/**
 * useNotice is what the section says unseen: say has it say text; the
 * uploads under parent that begin, here or dropped elsewhere, are said,
 * and those that leave uploaded. What leaves and begins in one turn is
 * said once the turn is over, together: each upload leaves by an action
 * of its own, and React would show only the last of several said in a
 * turn. The same text said again changes the region all the same, a space
 * after it, or it would not be read again.
 */
function useNotice(assets: AssetStore, parent: string | null) {
  const t = useT();
  const { preferences } = useStore();
  const [said, setSaid] = useState({ text: "", again: false });
  useEffect(() => {
    const names = new Intl.ListFormat(preferences.locale, { type: "conjunction" });
    // What the turn says, as it is so far: those left uploaded, and how many began.
    let left: string[] = [];
    let begun = 0;
    let text: string | undefined;
    let turn: ReturnType<typeof setTimeout> | undefined;
    const stop = reaction(
      () => assets.uploads.filter((upload) => upload.parent === parent),
      (now, before) => {
        for (const upload of before) {
          if (!now.includes(upload) && upload.uploaded !== undefined) {
            left.push(upload.name);
          }
        }
        begun += now.filter((upload) => !before.includes(upload) && upload.failure === undefined).length;
        const uploaded = left.length > 0 ? t("asset.uploaded", { names: names.format(left) }) : undefined;
        const uploading = begun > 0 ? t("asset.uploading", { count: begun }) : undefined;
        text =
          uploaded !== undefined && uploading !== undefined
            ? t("page.sentences", { first: uploaded, second: uploading })
            : (uploaded ?? uploading);
        turn ??= setTimeout(() => {
          if (text !== undefined) {
            setSaid(saying(text));
          }
          left = [];
          begun = 0;
          text = undefined;
          turn = undefined;
        }, 0);
      }
    );
    return () => {
      stop();
      clearTimeout(turn);
    };
  }, [assets, parent, t, preferences.locale]);
  return { text: said.text + (said.again ? "\u00A0" : ""), say: (text: string) => setSaid(saying(text)) };
}

/** saying is the notice that says text, after the one before: the same text is said again. */
function saying(text: string) {
  return (before: { text: string; again: boolean }) => ({ text, again: before.text === text && !before.again });
}

/**
 * useExpiry reads the list again before the first of its addresses expires, each time it is read (rereadIn); a hidden
 * tab's as it is shown, which SWR reads it again on.
 */
function useExpiry(list: AssetList | undefined, reread: () => void): void {
  const latest = useRef(reread);
  useEffect(() => {
    latest.current = reread;
  });
  useEffect(() => {
    if (list === undefined) {
      return undefined;
    }
    // An expiry the page cannot read is none: the others still have the list read again.
    let earliest = Number.POSITIVE_INFINITY;
    for (const asset of list.assets) {
      const expires = Date.parse(asset.expires_at);
      if (Number.isFinite(expires)) {
        earliest = Math.min(earliest, expires);
      }
    }
    if (!Number.isFinite(earliest)) {
      return undefined;
    }
    const timer = setTimeout(
      () => {
        if (!document.hidden) {
          latest.current();
        }
      },
      rereadIn(earliest, Date.now())
    );
    return () => clearTimeout(timer);
  }, [list]);
}
