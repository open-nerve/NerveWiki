import { Ellipsis, File, FileAudio, FileImage, FileText, FileVideo, type LucideIcon } from "lucide-react";
import type { Ref } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { formatBytes } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { assetKind, embedOf, opensInline, type AssetKind } from "../../lib/asset-kind";
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

/** What a row's menu asks its section for: a dialog of the attachment's, or the embed copied. */
export type AssetAction = "rename" | "move" | "delete" | "copy";

/**
 * AttachmentRow is an attachment of the list (M7/P4 design 3.5): its
 * kind's icon, its name, which opens it (in a tab of its own, said
 * unseen, for what the browser shows; downloaded otherwise), its size, and
 * its menu: Open, Download, Copy embed (for one a link leads to), and for
 * a writer Rename, Move to… and Delete, which the section holds. The row
 * drags as its embed, into the editor.
 */
export function AttachmentRow({
  asset,
  writer,
  act,
  linkRef,
}: {
  asset: Asset;
  writer: boolean;
  act: (action: AssetAction, asset: Asset) => void;
  linkRef?: Ref<HTMLAnchorElement>;
}) {
  const t = useT();
  const { preferences } = useStore();
  const Icon = icons[assetKind(asset.mime)];
  const inline = opensInline(asset.mime);
  const link = asset.link;
  const opening = inline
    ? { href: asset.content_url, target: "_blank", rel: "noopener noreferrer" }
    : { href: asset.download_url, download: asset.name };
  return (
    // oxlint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- dragged into the editor; Copy embed is the keyboard's way
    <li
      className="flex items-center gap-2 px-3 py-2 text-sm"
      draggable={link !== null || undefined}
      onDragStart={(event) => {
        if (link !== null) {
          event.dataTransfer.setData("text/plain", embedOf(link));
          event.dataTransfer.effectAllowed = "copy";
        }
      }}
    >
      <Icon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
      {/* A link drags itself: a drag from the name is the row's. */}
      <a ref={linkRef} {...opening} draggable={false} className="min-w-0 flex-1 truncate hover:underline">
        {asset.name}
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
            className="rounded p-1 text-muted-foreground hover:bg-accent"
          >
            <Ellipsis className="size-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
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
          {writer && (
            <>
              <DropdownMenuItem onSelect={() => act("rename", asset)}>{t("page.rename")}</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => act("move", asset)}>{t("page.moveTo")}</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => act("delete", asset)}>{t("page.delete")}</DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </li>
  );
}

/** NewTab says, unseen, that a link opens in a tab of its own: its space outside, which a name joins the words by. */
function NewTab() {
  const t = useT();
  return (
    <>
      {" "}
      <span className="sr-only">{t("asset.newTab")}</span>
    </>
  );
}
