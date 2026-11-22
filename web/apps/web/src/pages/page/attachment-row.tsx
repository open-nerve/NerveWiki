import { Ellipsis, File, FileAudio, FileImage, FileText, FileVideo, type LucideIcon } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, type Ref } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { formatBytes } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { assetKind, embedOf, opensInline, type AssetKind } from "../../lib/asset-kind";
import { focusLost } from "../../lib/focus-leaving";
import type { Asset } from "../../services/asset.service";
import { useStore } from "../../stores/context";

/** The icon of each kind of attachment: a type's, no preview (M7/P4 design 2). */
const icons: Record<AssetKind, LucideIcon> = {
  image: FileImage,
  audio: FileAudio,
  video: FileVideo,
  pdf: FileText,
  file: File,
};

/** AssetIcon is the icon of the kind of an attachment of type mime, for the eye alone: the trash's preview shows it too. */
export function AssetIcon({ mime }: { mime: string }) {
  const Icon = icons[assetKind(mime)];
  return <Icon aria-hidden className="size-4 shrink-0 text-muted-foreground" />;
}

/**
 * Located is the route state of a page or the notebook's home gone to for
 * one of its attachments (M8/P2 design 4.2): a restored one opened from
 * the trash, a search's result in P5. The row of the attachment takes the
 * focus, or the section's title when the list read has it not.
 */
export type Located = { attachment: string };

/** locatedOf is the attachment the route state names, if it names one. */
export function locatedOf(state: unknown): string | undefined {
  return typeof state === "object" && state !== null && "attachment" in state && typeof state.attachment === "string"
    ? state.attachment
    : undefined;
}

/** What a row's menu asks its section for: a dialog of the attachment's, or the embed copied. */
export type AssetAction = "rename" | "move" | "delete" | "copy";

/**
 * AttachmentRow is an attachment of the list (M7/P4 design 3.5): its
 * kind's icon, its name, which opens it (in a tab of its own, said
 * unseen, for what the browser shows; downloaded otherwise), its size, and
 * its menu: Open, Download, Copy embed (for one a link leads to), and for
 * whoever can manage it Rename, Move to… and Delete, which the section
 * holds, as it does where the focus goes as the menu closes (menuClosed).
 * The row drags as its embed, into the editor. Taken away, the list read
 * again without it, while the focus is in it or in its menu, which is a
 * portal outside it, it gives the focus to left: it would fall to the
 * body (M12/P2/S1, 16.1).
 */
export function AttachmentRow({
  asset,
  manage,
  deletable,
  source,
  act,
  menuClosed,
  linkRef,
  left,
}: {
  asset: Asset;
  manage: boolean;
  deletable: boolean;
  source: boolean;
  act: (action: AssetAction, asset: Asset) => void;
  menuClosed: (event: Event) => void;
  linkRef?: Ref<HTMLAnchorElement>;
  /** Where the focus goes as the row leaves the document holding it, or its menu does. */
  left: () => void;
}) {
  const t = useT();
  const row = useRef<HTMLLIElement>(null);
  // The menu's content as it last opened: closed, it is out of the document.
  const menu = useRef<HTMLElement | null>(null);
  const leaving = useRef(left);
  useEffect(() => {
    leaving.current = left;
  });
  // Whether the focus was in the row or its menu as it left: the layout effect's cleanup runs before the row's nodes,
  // and the menu's portal under it, leave the document, the focus still there.
  const held = useRef(false);
  useLayoutEffect(
    () => () => {
      const at = document.activeElement;
      held.current =
        row.current?.contains(at) === true || (menu.current?.isConnected === true && menu.current.contains(at));
    },
    []
  );
  // The passive effect's runs once they left: given then, the focus stays. Given earlier, the open menu's focus trap,
  // whose listeners go with its passive effects, would take it back to the item about to leave.
  useEffect(
    () => () => {
      if (held.current && focusLost()) {
        leaving.current();
      }
    },
    []
  );
  const { preferences } = useStore();
  const inline = opensInline(asset.mime);
  const link = asset.link;
  const opening = inline
    ? { href: asset.content_url, target: "_blank", rel: "noopener noreferrer" }
    : { href: asset.download_url, download: asset.name };
  return (
    // oxlint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- dragged into the editor; Copy embed is the keyboard's way
    <li
      ref={row}
      className="flex items-center gap-2 px-3 py-2 text-sm"
      draggable={link !== null || undefined}
      onDragStart={(event) => {
        if (link !== null) {
          event.dataTransfer.setData("text/plain", embedOf(link));
          event.dataTransfer.effectAllowed = "copy";
        }
      }}
    >
      <AssetIcon mime={asset.mime} />
      {source && <FileText aria-hidden className="size-4 shrink-0 text-muted-foreground" />}
      {/* A link drags itself: a drag from the name is the row's. */}
      <a
        ref={linkRef}
        {...opening}
        data-attachment={asset.id}
        draggable={false}
        className="relative min-w-0 flex-1 truncate hover:underline"
      >
        {asset.name}
        {source && (
          <>
            {" "}
            <span className="sr-only select-none">{t("page.sourceKind")}</span>
          </>
        )}
        {inline && <NewTab />}
      </a>
      <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
        {formatBytes(asset.byte_size, preferences.locale)}
      </span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            data-actions-of={asset.id}
            aria-label={t("asset.actions", { name: asset.name })}
            className="rounded p-1 text-muted-foreground hover:bg-hover"
          >
            <Ellipsis className="size-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          ref={(element) => {
            if (element !== null) {
              menu.current = element;
            }
          }}
          align="end"
          onCloseAutoFocus={menuClosed}
        >
          <DropdownMenuItem asChild>
            <a {...opening}>
              {t("asset.open")}
              {inline && <NewTab />}
            </a>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <a href={asset.download_url} download={asset.name}>
              {t("asset.download")}
            </a>
          </DropdownMenuItem>
          {link !== null && (
            <DropdownMenuItem onSelect={() => act("copy", asset)}>{t("asset.copyEmbed")}</DropdownMenuItem>
          )}
          {manage && (
            <>
              <DropdownMenuItem onSelect={() => act("rename", asset)}>{t("page.rename")}</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => act("move", asset)}>{t("page.moveTo")}</DropdownMenuItem>
            </>
          )}
          {deletable && (
            <>
              {/* Open and Download come first, always: the line sets Delete apart. */}
              <DropdownMenuSeparator />
              <DropdownMenuItem tone="destructive" onSelect={() => act("delete", asset)}>
                {t("page.delete")}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </li>
  );
}

/**
 * NewTab says, unseen, that a link opens in a tab of its own: its space outside, which a name joins the words by; not
 * copied with the text (M12/P2/S5, 17.2b).
 */
function NewTab() {
  const t = useT();
  return (
    <>
      {" "}
      <span className="sr-only select-none">{t("asset.newTab")}</span>
    </>
  );
}
