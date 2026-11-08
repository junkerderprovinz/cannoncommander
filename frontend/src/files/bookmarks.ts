// Shortcuts in the sidebar, and links to a folder that open it for whoever has the rights. Every change goes to the
// server right away, so the shortcuts follow the person to the next device.
import { api } from '@/api/client';
import type { Bookmark } from '@/api/types';
import { rootName } from '@/core/host';
import { toast } from '@/core/toast';
import { t } from '@/i18n';
import { useListing } from '@/stores/listing';
import { useUser } from '@/stores/user';
import { inArchive, split } from './paths';

const save = () => api.saveBookmarks(useUser().bookmarks).catch(() => toast(t('side.notSaved'), 'warn'));

export function addBookmark(path: string) {
  const user = useUser(), label = useListing().placeLabel(path);
  if (inArchive(path)) return toast(t('side.archiveNoBookmark'), 'warn');
  if (user.bookmarks.some((b) => b.path === path)) return toast(t('side.bookmarkExists'), 'warn');
  const name = label || split(path).pop() || rootName();
  const g = label ? (path === '/boot' ? 'IconFlash' : path === '/remotes' ? 'IconCloud' : 'IconZFS') : 'IconFolder';
  user.bookmarks.push({ name, path, g });
  save();
  toast(t('side.bookmarkAdded', { name }));
}

export function unpin(b: Bookmark) {
  const user = useUser(), i = user.bookmarks.indexOf(b);
  user.bookmarks.splice(i, 1);
  save();
  toast(t('side.unpinned', { name: b.name }), 'ok', { label: t('core.undo'), run: () => { user.bookmarks.splice(i, 0, b); save(); } });
}

export function moveBookmark(b: Bookmark, d: number) {
  const user = useUser(), i = user.bookmarks.indexOf(b), to = i + d;
  if (to < 0 || to >= user.bookmarks.length) return;
  user.bookmarks.splice(to, 0, user.bookmarks.splice(i, 1)[0]!);
  save();
}

export function renameBookmark(b: Bookmark, name: string) {
  b.name = name;
  save();
}

export function copyInternalLink(path: string) {
  const url = location.href.split('#')[0] + '#' + encodeURI(path);
  navigator.clipboard?.writeText(url).catch(() => {});
  toast(t('files.linkCopied'));
}
