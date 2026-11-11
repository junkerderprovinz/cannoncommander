// What is new in this version, each point with a way to get there. A point counts as seen once visited.
import { run } from '@/core/actions';
import { prefs } from '@/core/prefs';
import { go } from '@/core/views';
import { c } from '@/features/config/shared';

export interface News { id: string; g: string; text: () => string; go: () => void }
export const NEWS: News[] = [
  { id: 'oled', g: 'IconMoon', text: () => c('look.oledTip'), go: () => go('settings', 'look') },
  { id: 'files', g: 'IconFiles', text: () => c('news.files'), go: () => go('files') },
  { id: 'look', g: 'IconTabLook', text: () => c('news.look'), go: () => go('settings', 'look') },
  { id: 'users', g: 'IconAccounts', text: () => c('news.users'), go: () => go('settings', 'users') },
  { id: 'rules', g: 'IconShieldOn', text: () => c('news.rules'), go: () => go('settings', 'rules') },
  { id: 'remotes', g: 'IconCloud', text: () => c('news.remotes'), go: () => go('settings', 'remotes') },
  { id: 'shares', g: 'IconLink', text: () => c('news.shares'), go: () => go('settings', 'shares') },
  { id: 'security', g: 'IconTabSecurity', text: () => c('news.security'), go: () => go('settings', 'security') },
  { id: 'activity', g: 'IconLog', text: () => c('news.activity'), go: () => run('openActivity') },
];

export const seen = () => (prefs.newsSeen as string[]) || [];
export const unseen = () => NEWS.filter((n) => !seen().includes(n.id)).length;
