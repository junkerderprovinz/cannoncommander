// Strg+V with a picture or a text from another program: the clipboard's content becomes a file in the open folder,
// after a look at it and a name. Names cut or copied in the panels go first, as Krusader's own clipboard. It listens
// to the paste event rather than the key, as only the event carries what other programs put on the clipboard.
import './paste.css';
import { reactive } from 'vue';
import { run } from '@/core/actions';
import { toast } from '@/core/toast';
import { view } from '@/core/views';
import { cancelBtn, win, winStack } from '@/core/windows';
import { t } from '@/i18n';
import { useClipboard } from '@/stores/clipboard';
import { usePanels } from '@/stores/panels';
import { freeName, namesIn } from '@/ui/tools';
import { addFiles } from '@/features/upload/queue';
import PasteBody from './PasteBody.vue';

const pad = (n: number) => String(n).padStart(2, '0');
const stamp = (d = new Date()) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}-${pad(d.getMinutes())}`;

export default function setup() {
  const F = usePanels(), clip = useClipboard();
  document.addEventListener('paste', (e) => {
    if (view.id !== 'files' || winStack.length || (e.target as Element | null)?.closest?.('input, textarea, select, [contenteditable]')) return;
    if (clip.clip) { e.preventDefault(); run('clipPaste'); return; }
    const data = e.clipboardData;
    const pic = [...(data?.files || [])].find((f) => f.type.startsWith('image/'));
    const text = data?.getData('text/plain');
    if (!pic && !text) { toast(t('pastefile.empty'), 'warn'); return; }
    e.preventDefault();
    const folder = F.T().path;
    if (folder.includes('::')) { toast(t('pastefile.archive'), 'warn'); return; }
    const ext = pic ? (pic.type.split('/')[1] || 'png').replace('jpeg', 'jpg') : 'txt';
    const st = reactive({ name: freeName(namesIn(folder), pic ? t('pastefile.shot', { stamp: stamp(), ext }) : t('pastefile.note')) });
    const url = pic ? URL.createObjectURL(pic) : undefined;
    win({
      title: pic ? t('pastefile.picture') : t('pastefile.text'), body: PasteBody, props: { st, url, text: pic ? undefined : text, folder },
      foot: [cancelBtn(), { label: t('pastefile.create'), g: 'IconPaste', primary: true, run: () => {
        const name = st.name.trim();
        if (!name) return false;
        if (namesIn(folder).has(name)) { toast(t('pastefile.taken'), 'warn'); return false; }
        void addFiles([pic ? new File([pic], name, { type: pic.type }) : new File([text!], name, { type: 'text/plain' })], folder);
        toast(t('pastefile.made', { name }));
        return true;
      } }],
      onClose: () => { if (url) URL.revokeObjectURL(url); },
    });
  });
}
