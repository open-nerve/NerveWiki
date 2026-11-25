import { reaction } from "mobx";
import { Upload as UploadIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { useLocation } from "react-router";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { writesNotebook } from "../../app/effective-role";
import { useFileDrop } from "../../app/file-drop";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { cardClass } from "../../app/settings-page";
import { canAddSource, canDeleteNode, canManageNode } from "../../app/source-permissions";
import { FormField } from "../../components/form-field";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { formatPeriod } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Locale } from "../../i18n/locale";
import { embedOf } from "../../lib/asset-kind";
import { cn } from "../../lib/cn";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { ApiError } from "../../services/api";
import type { Asset } from "../../services/asset.service";
import type { Notebook } from "../../services/notebook.service";
import type { AssetList, AssetStore, Upload } from "../../stores/asset.store";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { MoveAssetDialog, RenameAssetDialog } from "./asset-dialogs";
import { dueWatch, rereadIn } from "./assets-expiry";
import { AttachmentRow, locatedOf, type AssetAction } from "./attachment-row";
import { watchReader } from "./readers-input";
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
 * row's menu, the section's title when the row is gone. A deletion says
 * the attachment goes to the trash, for how long (M8/P2 design 4.5). Gone
 * to for one of its attachments (Located), the section gives its row the
 * focus once a list read since it showed has it, or its title once one
 * has not, saying so when that list is whole: the attachment is gone
 * (useLocated).
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
  const pages = usePageTree(notebook);
  const { instance, preferences } = useStore();
  useSWR("instance", () => instance.load());
  const retention = instance.info?.trash_retention_seconds;
  const { error, mutate } = useSWR(["assets", notebook.id, parent ?? "root"], () => assets.load(parent));
  const list = assets.listOf(parent);
  useExpiry(list, () => void mutate());
  const section = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const gone = useLocated(list, section, heading, () => void mutate().catch(() => undefined));
  const goneNote = useId();
  const writer = writesNotebook(notebook);
  const uploads = assets.uploads.filter(
    (upload) => upload.parent === parent && !(editorsElsewhere && upload.fromEditor)
  );
  const headingId = useId();
  const uploadButton = useRef<HTMLButtonElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const [folders, setFolders] = useState(false);
  // The attachment whose embed was copied, or shown to copy by hand: by its id, its embed and name as listed now.
  const [copied, setCopied] = useState<{ id: string; done: boolean } | undefined>(undefined);
  const copyField = useRef<HTMLInputElement>(null);
  // Whether the menu closing chose Copy embed: the field of an embed not copied takes the focus then.
  const copying = useRef(false);
  const notice = useNotice(assets, parent);
  // The attachment of the dialog last opened, which stays as it closes, and which dialog is open.
  const [target, setTarget] = useState<Asset | undefined>(undefined);
  const [dialog, setDialog] = useState<Dialog | undefined>(undefined);
  const source = canAddSource(pages, parent);
  const upload = (files: readonly File[]) => {
    if (pages.notebook === undefined || !writesNotebook(pages.notebook)) return;
    if (parent !== null && canAddSource(pages, parent)) {
      assets.uploadSources(parent, files, { untitled: t("asset.untitled"), maxBytes: instance.info?.asset_max_bytes });
    } else {
      assets.upload(parent, files, t("asset.untitled"), { maxBytes: instance.info?.asset_max_bytes });
    }
  };
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

  // The embed shown to copy by hand follows its attachment renamed elsewhere, and goes as it goes, or loses its
  // extension (M12/P2/S5, 16.4b).
  const copiedAsset =
    copied !== undefined && !copied.done ? list?.assets.find((asset) => asset.id === copied.id) : undefined;
  const copiedLink = copiedAsset?.link ?? null;
  // Gone, it is done with: the attachment back (restored, renamed back, read by More) does not bring the field back
  // to take the focus from where the reader is by then.
  const copiedGone = copied !== undefined && !copied.done && list !== undefined && copiedLink === null;
  useEffect(() => {
    if (copiedGone) {
      setCopied(undefined);
    }
  }, [copiedGone]);

  if (
    !writer &&
    uploads.length === 0 &&
    !gone &&
    (list === undefined ? error === undefined : list.assets.length === 0)
  ) {
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
            setCopied({ id: asset.id, done: false });
            return;
          }
          setCopied({ id: asset.id, done: true });
          notice.say(t("asset.copied"));
        };
        void copy();
      }
      return;
    }
    if (action === "delete" ? !canDeleteNode(pages, asset.id) : !canManageNode(pages, asset.id)) {
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
      // A drag of files that may drop here rings it amber, a state (M13/P5/S3, decision 933).
      className={cn("space-y-2 rounded-card", drop.over && "ring-2 ring-accent ring-offset-4 ring-offset-background")}
      {...drop.handlers}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2
          id={headingId}
          ref={heading}
          tabIndex={-1}
          aria-describedby={gone ? goneNote : undefined}
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
            <Button ref={uploadButton} variant="outline" size="sm" shape="pill" onClick={() => picker.current?.click()}>
              <UploadIcon />
              {t(source ? "source.uploadFiles" : "asset.upload")}
            </Button>
          </>
        )}
      </div>
      {gone && (
        <output id={goneNote} className="block text-sm">
          {t("asset.locatedGone")}
        </output>
      )}
      {/* Said as it changes: what began, what was uploaded, an embed copied. */}
      <p aria-live="polite" className="sr-only">
        {notice.text}
      </p>
      {writer && list !== undefined && list.assets.length === 0 && uploads.length === 0 && (
        <p className="text-compact text-faint">{t("asset.dropHint")}</p>
      )}
      {folders && <Alert>{t("asset.folders")}</Alert>}
      {copied?.done === true && <p className="text-sm text-muted-foreground">{t("asset.copied")}</p>}
      {copied !== undefined && copiedAsset !== undefined && copiedLink !== null && (
        <CopyField
          key={copied.id}
          embed={embedOf(copiedLink)}
          name={copiedAsset.name}
          field={copyField}
          left={() => heading.current?.focus()}
        />
      )}
      {uploads.length > 0 && (
        <UploadRows
          notebook={notebook}
          assets={assets}
          uploads={uploads}
          left={() => (uploadButton.current ?? heading.current)?.focus()}
        />
      )}
      {list === undefined ? (
        <NotLoaded error={error} retry={() => void mutate()} />
      ) : (
        list.assets.length > 0 && (
          // A card of rows, as a list's page's (decision 933).
          <ul aria-labelledby={headingId} className={cardClass}>
            {list.assets.map((asset) => (
              <AttachmentRow
                key={asset.id}
                asset={asset}
                manage={canManageNode(pages, asset.id)}
                deletable={canDeleteNode(pages, asset.id)}
                source={pages.isSource(asset.id)}
                act={act}
                menuClosed={menuClosed}
                linkRef={asset.id === more.focusing ? focused : undefined}
                left={() => heading.current?.focus()}
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
            description={
              retention === undefined
                ? t("asset.deleteBodyNoPeriod")
                : t("asset.deleteBody", { period: formatPeriod(retention, preferences.locale) })
            }
            confirmLabel={t("page.delete")}
            sendingLabel={t("page.deleting")}
            cancelLabel={t("page.cancel")}
            confirm={async () => {
              const id = shown.id;
              function requireDelete() {
                if (!canDeleteNode(pages, id)) {
                  throw new ApiError(403, { code: "forbidden" });
                }
              }
              requireDelete();
              await assets.remove(id, shown.parent_id, requireDelete);
            }}
          />
        </>
      )}
    </section>
  );
});

/**
 * CopyField is an embed the page could not copy, the clipboard out of its
 * reach (a page served without HTTPS): in a field, selected as it is
 * focused, which it is as it shows, to copy by hand. Its embed and name
 * change as its attachment is renamed, the field kept; as it goes holding
 * the focus, the focus goes to left (v0.1 design 13.2, item 17).
 */
function CopyField({
  embed,
  name,
  field,
  left,
}: {
  embed: string;
  name: string;
  field: RefObject<HTMLInputElement | null>;
  left: () => void;
}) {
  const t = useT();
  useFocusLeaving(field, left);
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
 * turn. Those begun are counted then, as still going: one cancelled, or
 * failed, in its turn is not (M12/P2/S5, 16.10). The same text said again
 * changes the region all the same, a space after it, or it would not be
 * read again. Once none goes any more and the turn has nothing to say, a
 * sentence of uploads going no longer holds, and the region is emptied of
 * it: an upload failed or cancelled leaves no "Uploading" behind it; one
 * uploaded is said all the same (M12/P2/S5, 16.4c, 38.9). What say had it
 * say since, an embed copied, holds still, and stays (P2-40).
 */
function useNotice(assets: AssetStore, parent: string | null) {
  const t = useT();
  const { preferences } = useStore();
  const [said, setSaid] = useState({ text: "", again: false });
  // Whether the region says the uploads' words, not say's: only those are emptied.
  const uploadsSaid = useRef(false);
  useEffect(() => {
    const sentence = turnSentence(t, preferences.locale);
    // What the turn says, as it is so far: those left uploaded, those that began, and whether none goes any more.
    let left: string[] = [];
    const begun = new Set<Upload>();
    let ended = false;
    let turn: ReturnType<typeof setTimeout> | undefined;
    const flush = () => {
      // Read as the turn ends: an upload's signal is not observed.
      const going = [...begun].filter(
        (upload) => assets.uploads.includes(upload) && upload.failure === undefined && !upload.signal.aborted
      ).length;
      const text = sentence(left, going);
      if (text !== undefined) {
        uploadsSaid.current = true;
        setSaid(saying(text));
      } else if (ended && uploadsSaid.current) {
        uploadsSaid.current = false;
        setSaid({ text: "", again: false });
      }
      left = [];
      begun.clear();
      ended = false;
      turn = undefined;
    };
    const stopUploads = reaction(
      () => assets.uploads.filter((upload) => upload.parent === parent),
      (now, before) => {
        for (const upload of before) {
          if (!now.includes(upload) && upload.result !== undefined) {
            left.push(upload.name);
          }
        }
        for (const upload of now) {
          if (!before.includes(upload) && upload.failure === undefined) {
            begun.add(upload);
          }
        }
        turn ??= setTimeout(flush, 0);
      }
    );
    // A failure leaves the uploads as they are: whether any goes tells it.
    const stopGoing = reaction(
      () => assets.going.some((upload) => upload.parent === parent),
      (going) => {
        if (!going) {
          ended = true;
          turn ??= setTimeout(flush, 0);
        }
      }
    );
    return () => {
      stopUploads();
      stopGoing();
      clearTimeout(turn);
    };
  }, [assets, parent, t, preferences.locale]);
  return {
    text: said.text + (said.again ? "\u00A0" : ""),
    say: (text: string) => {
      uploadsSaid.current = false;
      setSaid(saying(text));
    },
  };
}

/**
 * turnSentence says what a turn of uploads says, in locale: those left uploaded, by name, and how many still go; or
 * nothing.
 */
function turnSentence(t: Translate, locale: Locale): (left: readonly string[], going: number) => string | undefined {
  const names = new Intl.ListFormat(locale, { type: "conjunction" });
  return (left, going) => {
    const uploaded = left.length > 0 ? t("asset.uploaded", { names: names.format(left) }) : undefined;
    const uploading = going > 0 ? t("asset.uploading", { count: going }) : undefined;
    return uploaded !== undefined && uploading !== undefined
      ? t("page.sentences", { first: uploaded, second: uploading })
      : (uploaded ?? uploading);
  };
}

/** saying is the notice that says text, after the one before: the same text is said again. */
function saying(text: string) {
  return (before: { text: string; again: boolean }) => ({ text, again: before.text === text && !before.again });
}

/**
 * useLocated gives the focus, once, to the row of the attachment the route
 * state names (Located), as soon as a list read since the section showed
 * has it: its link, in section. A list cached from before is not: the
 * attachment may have gone meanwhile, its row going with the focus as the
 * list is read again (M8 closeout B-M1), so that list is read again at
 * once (reread), SWR's own read on mount waiting out its deduplication.
 * A list read since that has it not gives the focus to the section's
 * title, heading: the attachment is on a later page; or, the list whole,
 * gone, which useLocated answers true for the section to say, its title
 * taking the focus once that shows. Not when the reader did something
 * since the section showed, or the focus is somewhere already (v0.1
 * design 13.2, item 26, as transfer-page.tsx's useArriving): it gives up
 * then, the attachment gone said all the same. The section shown again,
 * by Back say, does it again: the state is the entry's. It answers true
 * only while the list read, whole, has it not: restored elsewhere, its
 * row shows, or More may read it, and gone is said no more (M12/P2/S5,
 * 28.1).
 */
function useLocated(
  list: AssetList | undefined,
  section: RefObject<HTMLElement | null>,
  heading: RefObject<HTMLHeadingElement | null>,
  reread: () => void
): boolean {
  const located = locatedOf(useLocation().state);
  const shownFirst = useRef(list);
  const done = useRef(false);
  const reader = useRef<ReturnType<typeof watchReader> | undefined>(undefined);
  const latest = useRef(reread);
  useEffect(() => {
    latest.current = reread;
  });
  // Whether the attachment is gone, and the title is to take the focus as it says so.
  const [gone, setGone] = useState<{ focus: boolean } | undefined>(undefined);
  useEffect(() => {
    if (located === undefined) {
      return;
    }
    const watching = watchReader();
    reader.current = watching;
    if (shownFirst.current !== undefined) {
      latest.current();
    }
    return () => watching.end();
  }, [located]);
  useEffect(() => {
    if (located === undefined || list === undefined || list === shownFirst.current || done.current) {
      return;
    }
    done.current = true;
    const acted = reader.current?.acted() === true;
    reader.current?.end();
    const at = document.activeElement;
    const free = !acted && (at === null || at === document.body);
    if (list.next === null && !list.assets.some((asset) => asset.id === located)) {
      setGone({ focus: free });
      return;
    }
    if (free) {
      (section.current?.querySelector<HTMLElement>(`[data-attachment="${located}"]`) ?? heading.current)?.focus();
    }
  }, [located, list, section, heading]);
  useEffect(() => {
    if (gone?.focus === true) {
      heading.current?.focus();
    }
  }, [gone, heading]);
  // Only the focus is given once: gone is said as the list read now, whole, has it not, and no more once it has it
  // again, restored elsewhere, or is no longer whole, which a later page may hold it on (M12/P2/S5, 28.1).
  return gone !== undefined && list?.next === null && !list.assets.some((asset) => asset.id === located);
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
    // Watched, not timed alone: a machine that slept past it reads it again within a minute of waking (17.6).
    return dueWatch(Date.now() + rereadIn(earliest, Date.now()), () => {
      if (!document.hidden) {
        latest.current();
      }
    });
  }, [list]);
}
