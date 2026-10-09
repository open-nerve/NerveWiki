import { reaction } from "mobx";
import { Upload as UploadIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type ReactNode, type RefObject } from "react";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { writesPages } from "../../app/effective-role";
import { useFileDrop } from "../../app/file-drop";
import { useMounted } from "../../app/mounted";
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
import { MoveAssetDialog, RenameAssetDialog } from "./asset-dialogs";
import { AttachmentRow, type AssetAction } from "./attachment-row";
import { watchReader } from "./readers-input";
import { UploadRows } from "./upload-rows";

/** The dialogs of the section's rows. */
type Dialog = Exclude<AssetAction, "copy">;

/** How long before the first of its addresses expires a list is read again: they are signed anew. */
const expiryMargin = 60_000;
/** How soon at the earliest: a clock far from the server's would read the list again and again. */
const expiryFloor = 30_000;
/** How long an address is good for at the least, from as it is read: the server signs it for an hour or more. */
const signedFor = 60 * 60_000;

/**
 * AttachmentsSection is the attachments under a page, in its middle column
 * below its subpages, or under the notebook's root, on its home (M7/P4
 * design 3.5): each a row (AttachmentRow), a hundred at a time, More
 * reading the next hundred. A reader sees it only when there are some. A
 * writer uploads files, by Upload or dropped on it (a folder is not: it
 * says to import one), and their uploads show above the list (UploadRows);
 * what began and what was uploaded are said, unseen. The list is read
 * again before the first of its addresses expires: a link followed must be
 * signed still, and one signed on a click would open a tab the browser
 * takes for a pop-up. Copy embed copies a row's embed; where the page has
 * no clipboard, the embed shows in a field to copy from. The section holds
 * the dialogs of its rows, which a row read away (the rename done, another
 * tab's) does not take with it; one closed gives the focus back to its
 * row's menu, the section's title when the row is gone.
 */
export const AttachmentsSection = observer(function AttachmentsSection({
  notebook,
  parent,
}: {
  notebook: Notebook;
  /** The page whose attachments these are; null for the notebook's root. */
  parent: string | null;
}) {
  const t = useT();
  const assets = useAssets(notebook);
  const { instance } = useStore();
  useSWR("instance", () => instance.load());
  const { error, mutate } = useSWR(["assets", notebook.id, parent ?? "root"], () => assets.load(parent));
  const list = assets.listOf(parent);
  useExpiry(list, () => void mutate());
  const writer = writesPages(notebook.role);
  const uploads = assets.uploads.filter((upload) => upload.parent === parent);
  const headingId = useId();
  const section = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const uploadButton = useRef<HTMLButtonElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const [folders, setFolders] = useState(false);
  const [copied, setCopied] = useState<{ embed: string; name: string; done: boolean } | undefined>(undefined);
  const notice = useNotice(assets, parent);
  // The attachment of the dialog last opened, which stays as it closes, and which dialog is open.
  const [target, setTarget] = useState<Asset | undefined>(undefined);
  const [dialog, setDialog] = useState<Dialog | undefined>(undefined);
  const upload = (files: readonly File[]) => {
    const started = assets.upload(parent, files, t("asset.untitled"), { maxBytes: instance.info?.asset_max_bytes });
    const going = started.filter((each) => each.failure === undefined).length;
    if (going > 0) {
      notice.say(t("asset.uploading", { count: going }));
    }
  };
  const drop = useFileDrop(writer, (dropped) => {
    setFolders(dropped.folders);
    upload(dropped.files);
  });
  const more = useMore(assets, parent, heading);

  if (!writer && uploads.length === 0 && (list === undefined ? error === undefined : list.assets.length === 0)) {
    return null;
  }

  function act(action: AssetAction, asset: Asset) {
    setCopied(undefined);
    if (action === "copy") {
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
          <CopyField key={copied.embed} embed={copied.embed} name={copied.name} />
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
                linkRef={asset.id === more.focusing ? more.focused : undefined}
              />
            ))}
          </ul>
        )
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {list?.next != null && (
        <Button
          ref={more.button}
          variant="outline"
          aria-busy={more.reading || undefined}
          aria-disabled={more.reading || undefined}
          onClick={() => void more.read()}
        >
          {t("asset.more")}
        </Button>
      )}
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
function CopyField({ embed, name }: { embed: string; name: string }) {
  const t = useT();
  const field = useRef<HTMLInputElement>(null);
  useEffect(() => {
    // After the menu it was copied from gives the focus back to its button;
    // each copy that fails shows it anew, the copy before cleared.
    const timer = setTimeout(() => field.current?.focus(), 0);
    return () => clearTimeout(timer);
  }, []);
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
 * AttachmentDrop is where files dropped upload under parent, as on its
 * attachments' section: the reading view of a page, for a writer (M7/P4
 * design 3.6). A folder is not uploaded: it says to import one.
 */
export function AttachmentDrop({
  notebook,
  parent,
  enabled,
  children,
}: {
  notebook: Notebook;
  parent: string | null;
  enabled: boolean;
  children: ReactNode;
}) {
  const t = useT();
  const assets = useAssets(notebook);
  const { instance } = useStore();
  const [folders, setFolders] = useState(false);
  const drop = useFileDrop(enabled, (dropped) => {
    setFolders(dropped.folders);
    assets.upload(parent, dropped.files, t("asset.untitled"), { maxBytes: instance.info?.asset_max_bytes });
  });
  return (
    <div className={cn("space-y-4 rounded-md", drop.over && "ring-2 ring-primary ring-offset-4")} {...drop.handlers}>
      {folders && <Alert>{t("asset.folders")}</Alert>}
      {children}
    </div>
  );
}

/**
 * useNotice is what the section says unseen: say has it say text; the
 * uploads under parent that leave uploaded are said too, those that leave
 * together in one.
 */
function useNotice(assets: AssetStore, parent: string | null) {
  const t = useT();
  const [text, setText] = useState("");
  useEffect(
    () =>
      reaction(
        () => assets.uploads.filter((upload) => upload.parent === parent),
        (now, before) => {
          const uploaded = before.filter((upload) => !now.includes(upload) && upload.uploaded !== undefined);
          if (uploaded.length > 0) {
            setText(t("asset.uploaded", { names: uploaded.map((upload) => upload.name).join(", ") }));
          }
        }
      ),
    [assets, parent, t]
  );
  return { text, say: setText };
}

/**
 * useExpiry reads the list again expiryMargin before the first of its
 * addresses expires, each time it is read: expiryFloor at the soonest, and
 * at the latest before an hour from the read is over, which an address is
 * good for whatever the clocks say.
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
    let earliest = Number.POSITIVE_INFINITY;
    for (const asset of list.assets) {
      earliest = Math.min(earliest, Date.parse(asset.expires_at));
    }
    if (!Number.isFinite(earliest)) {
      return undefined;
    }
    const wait = Math.min(Math.max(earliest - Date.now() - expiryMargin, expiryFloor), signedFor - expiryMargin);
    const timer = setTimeout(() => latest.current(), wait);
    return () => clearTimeout(timer);
  }, [list]);
}

/**
 * useMore reads the next page of the attachments under parent. As the
 * last is read, More goes: the focus falls to the first attachment it
 * added, or the list's last, or the section's title, unless the reader did
 * something meanwhile or the focus is elsewhere than where More was (v0.1
 * design 13.2, item 26).
 */
function useMore(assets: AssetStore, parent: string | null, heading: RefObject<HTMLHeadingElement | null>) {
  const mounted = useMounted();
  const [reading, setReading] = useState(false);
  const [failure, setFailure] = useState<unknown>(undefined);
  // Where the focus falls as More goes: an attachment's link, or the section's title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const busy = useRef(false);
  const button = useRef<HTMLButtonElement>(null);
  const focused = useRef<HTMLAnchorElement>(null);
  useEffect(() => {
    if (focusing === undefined) {
      return;
    }
    const at = document.activeElement;
    if (at === null || at === document.body) {
      (focusing === null ? heading.current : focused.current)?.focus({ preventScroll: true });
    }
  }, [focusing, heading]);

  async function read(): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    setFocusing(undefined);
    const reader = watchReader({ on: button.current });
    try {
      const added = await assets.more(parent);
      const list = assets.listOf(parent);
      const at = document.activeElement;
      const held = !reader.acted() && (at === button.current || at === null || at === document.body);
      if (mounted() && held && list !== undefined && list.next === null) {
        setFocusing(added[0]?.id ?? list.assets.at(-1)?.id ?? null);
      }
    } catch (failed) {
      if (mounted()) {
        setFailure(failed);
      }
    } finally {
      reader.end();
      busy.current = false;
      if (mounted()) {
        setReading(false);
      }
    }
  }

  return { reading, failure, focusing, focused, button, read };
}
