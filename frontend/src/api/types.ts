// The wire shapes of /api/v1. Once the OpenAPI document exists these come from the
// generator, and only client.ts has to follow.
export interface FsItem {
  name: string;
  dir: boolean;
  size: number;
  // A folder the index has not measured yet, whose size of 0 is no total.
  sizeUnknown?: boolean;
  mtime: string;
  mode: string;
  owner?: string;
  group?: string;
  hidden?: boolean;
  // A protected file of the system, which stays hidden until that is asked for as well.
  system?: boolean;
  // The attributes of Windows with the letters of Explorer's column, such as RA.
  attrs?: string;
  // A file a cloud provider keeps: only online, on this device, or always kept on it.
  cloud?: 'online' | 'local' | 'pinned';
  // Where a .lnk, .url or .desktop file leads; path is set when the target lies on a source.
  shortcut?: Shortcut;
  // The name Explorer shows for a folder, such as Dokumente for Documents.
  label?: string;
  link?: string;
  items?: number;
  // The server can list what is inside, so the archive opens like a folder.
  archive?: boolean;
  btime?: string;
  ino?: number;
  nlink?: number;
  uid?: number;
  gid?: number;
  // A folder's poster.jpg, folder.jpg or cover.jpg, which the card views show for it.
  picture?: string;
}
export interface Shortcut { kind: 'lnk' | 'url' | 'desktop'; target?: string; path?: string; dir?: boolean; args?: string; name?: string; icon?: string }
// kind and remote come with a drive of the computer the desktop app runs on, as on TreeItem.
export interface Volume { path: string; label?: string; kind?: DriveKind; remote?: string; free: number; total: number }
export interface FsList { path: string; items: FsItem[]; volume?: Volume }
// kind and eject come with a drive of the computer the desktop app runs on.
export interface TreeItem { name: string; path: string; hasDirs: boolean; label?: string; free?: number; total?: number; remote?: string; hidden?: boolean; system?: boolean; kind?: DriveKind; eject?: boolean }
export type DriveKind = 'fixed' | 'removable' | 'optical' | 'network' | 'virtual';
export interface FsTree { path: string; items: TreeItem[] }
// fs/meta answers per path with the asked columns it knows. Numbers are bytes, milliseconds, seconds of play or plain
// figures as the column says; booleans are yes or no; text arrives ready to show.
export interface FsMetaRequest { paths: string[]; columns: string[] }
export interface FsMeta { items: Record<string, Record<string, string | number | boolean | null>> }
export interface FsText { text: string; encoding?: string }
// A hit carries the folder it was found in.
export interface SearchHit extends FsItem { at: string }
export interface SearchResult { path: string; items: SearchHit[]; total: number; more?: boolean; scanned: number; pending?: number; unknown?: string[]; indexed: boolean }
export interface WatchRequest { clientId: string; paths: string[] }
// home is the folder the file manager opens when no start folder was picked.
export interface Me { name: string; full: string; role: string; home?: string }
export interface Bookmark { name: string; path: string; g: string }
// code and params name the sentence in detail, for the interface to say it in its own language.
export interface Problem { type: string; title: string; status: number; detail?: string; code?: string; params?: Record<string, unknown> }
