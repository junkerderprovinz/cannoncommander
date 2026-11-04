import { reactive } from 'vue';

// Wrapping and the markdown preview stay as they were left, from one file to the next.
export const opts = reactive({ wrap: false, split: true });

// What the window's foot and its keys need from the body.
export interface EditorState { dirty: boolean; save: () => Promise<boolean>; ask: () => void }
