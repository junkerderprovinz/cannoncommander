// Hochladen per button, and files dropped onto a panel from the desktop.
import { reactive } from 'vue';
import { action } from '@/core/actions';
import { closeBtn, win, type WinEntry } from '@/core/windows';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import { addFiles, Up } from './queue';
import UploadBody from './UploadBody.vue';
import './upload.css';

let open: WinEntry | null = null;
const st = reactive({ path: '/' });

function uploadWindow(path: string) {
  st.path = path;
  if (open) return open;
  Up.shown = true;
  const pick = (id: string) => (el: HTMLElement) => { el.querySelector<HTMLInputElement>(id)?.click(); return false; };
  open = win({
    title: t('upload.title'), wide: true, body: UploadBody, props: { st },
    foot: [{ start: t('upload.keepsRunning') }, closeBtn(), { label: t('upload.pickFolder'), g: 'IconFolder', run: pick('#cu-din') }, { label: t('upload.pickFiles'), g: 'IconFile', primary: true, run: pick('#cu-in') }],
    onClose: () => { open = null; Up.shown = false; },
  });
  return open;
}

export default function setup() {
  const F = usePanels();
  action('upload', { label: tl('upload.title'), g: 'IconUpload', needs: 'writable', run: () => uploadWindow(F.T().path) });
  action('uploadFiles', { label: tl('upload.title'), g: 'IconUpload', hidden: true, run: (files: File[], path: string) => {
    uploadWindow(path);
    return addFiles(files, path);
  } });
}
