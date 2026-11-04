// The wire shapes of /api/v1. Once the OpenAPI document exists these come from the
// generator, and only client.ts has to follow.
export interface FsItem {
  name: string;
  dir: boolean;
  size: number;
  mtime: string;
  mode: string;
  owner?: string;
  group?: string;
  hidden?: boolean;
  link?: string;
  items?: number;
}
export interface Volume { path: string; label?: string; free: number; total: number }
export interface FsList { path: string; items: FsItem[]; volume?: Volume }
export interface TreeItem { name: string; path: string; hasDirs: boolean; label?: string; free?: number; total?: number; remote?: string }
export interface FsTree { path: string; items: TreeItem[] }
export interface Me { name: string; full: string; role: string }
export interface Bookmark { name: string; path: string; g: string }
export interface TrashItem { name: string; from: string; size: number; deleted: string; by: string; dir?: boolean }
export interface Problem { type: string; title: string; status: number; detail?: string }
