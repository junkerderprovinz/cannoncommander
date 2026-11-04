// Which files the browser can show by itself, so the page offers a preview only where one works.
import { type Kind, kindOf } from '@/core/glyphs';

const SHOWN: Kind[] = ['img', 'video', 'audio', 'pdf', 'text', 'code'];

export const canPreview = (name: string) => SHOWN.includes(kindOf({ name }));
export const previewKind = (name: string) => kindOf({ name });
