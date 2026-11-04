// What every renderer gets and gives back. A renderer draws one file, either large in the viewer window or compact
// in the preview bar and the quick look; it tells the frame a line about the file and may take keys.
import type { FileNode } from '../kinds';

export interface RProps {
  n: FileNode;
  folder: string;
  // Files the viewer can step through, such as the pictures of a folder; just the file itself elsewhere.
  sibs: FileNode[];
  compact?: boolean;
  autoplay?: boolean;
}

export interface REmits {
  (e: 'info', text: string): void;
  (e: 'go', to: FileNode, play?: boolean): void;
}

// True means the renderer used the key.
export type KeyHandler = (e: KeyboardEvent, combo: string) => boolean | void;
export interface RExposed { keys?: KeyHandler }
