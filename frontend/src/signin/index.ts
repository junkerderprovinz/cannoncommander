// The sign in: its texts, whether the session ended, and the action that signs out.
import { reactive } from 'vue';
import { auth } from '@/api/auth';
import { onSignedOut } from '@/api/client';
import { disconnect } from '@/api/sse';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { addMessages, t, tl } from '@/i18n';
import { useUser } from '@/stores/user';
import de from './de.json';
import en from './en.json';
import './signin.css';

addMessages('signin', { de, en });

export const session = reactive({ out: false });

// The note the page shows after a reload, such as the greeting after the sign in.
const NOTE = 'cannoncommander-signin-note';
export function leaveNote(text: string) {
  try { sessionStorage.setItem(NOTE, text); } catch { /* the page then simply greets nobody */ }
}
export function takeNote(): string {
  try {
    const text = sessionStorage.getItem(NOTE) || '';
    sessionStorage.removeItem(NOTE);
    return text;
  } catch {
    return '';
  }
}

export function installSignOut() {
  onSignedOut(() => { session.out = true; });
  action('logout', {
    label: tl('signin.logout'), g: 'IconSignOut',
    // The desktop app signs nobody in, and neither does a server where nobody has a password yet.
    when: () => !host.desktop && useUser().me.signIn === true,
    run: async () => {
      await auth.logout().catch(() => {});
      disconnect();
      leaveNote(t('signin.bye'));
      location.reload();
    },
  });
}
