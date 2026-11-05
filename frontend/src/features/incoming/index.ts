// Files shared from another app on the phone: sw.js takes them and opens the app with ?geteilt, and the app asks
// where they go before it hands them to the upload.
import './incoming.css';
import { reactive } from 'vue';
import { exists } from '@/api/admin';
import { run } from '@/core/actions';
import { now } from '@/core/format';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { t } from '@/i18n';
import { usePanels } from '@/stores/panels';
import IncomingBody from './IncomingBody.vue';
import { takeReceived } from './received';

// Browsers only run a service worker on https or on localhost.
function register() {
  if (!('serviceWorker' in navigator) || !isSecureContext) return;
  navigator.serviceWorker.register(import.meta.env.BASE_URL + 'sw.js').catch(() => {});
}

function takeFlag() {
  const params = new URLSearchParams(location.search);
  if (!params.has('geteilt')) return false;
  params.delete('geteilt');
  const rest = params.toString();
  history.replaceState(history.state, '', location.pathname + (rest ? '?' + rest : '') + location.hash);
  return true;
}

async function receive() {
  const files = await takeReceived(t('incoming.noteName'), now()).catch(() => []);
  if (!files.length) return toast(t('incoming.nothing'), 'warn');
  const here = usePanels().T().path;
  const st = reactive({ path: here.includes('::') ? '/mnt/user' : here });
  const go = async (el: HTMLElement) => {
    const path = st.path.trim();
    if (!(await exists(path))) {
      toast(t('incoming.noFolder'), 'warn');
      el.querySelector<HTMLInputElement>('#ci-target')?.focus();
      return;
    }
    w.close();
    run('uploadFiles', files, path);
  };
  const w = win({
    title: t('incoming.title'), body: IncomingBody, props: { files, st },
    foot: [cancelBtn(), { label: t('incoming.upload'), g: 'IconUpload', primary: true, run: (el) => { void go(el); return false; } }],
  });
}

export default function setup() {
  register();
  if (takeFlag()) void receive();
}
