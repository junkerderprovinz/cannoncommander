// The settings tabs, the activity page and the help actions that lead into them.
import './config.css';
import { action } from '@/core/actions';
import { host } from '@/core/host';
import { registerView, settingsTab, go } from '@/core/views';
import { win } from '@/core/windows';
import { admin } from '@/api/admin';
import { tl } from '@/i18n';
import ActivityPage from './ActivityPage.vue';
import { c } from '@/features/config/shared';
import { takeAgainFromUrl } from './proof';
import Apps from './tabs/Apps.vue';
import FileList from './tabs/FileList.vue';
import General from './tabs/General.vue';
import Info from './tabs/Info.vue';
import Integrations from './tabs/Integrations.vue';
import Look from './tabs/Look.vue';
import Remotes from './tabs/Remotes.vue';
import Rules from './tabs/Rules.vue';
import Security from './tabs/Security.vue';
import Shares from './tabs/Shares.vue';
import Sources from './tabs/Sources.vue';
import System from './tabs/System.vue';
import Users from './tabs/Users.vue';
import DocsWin from './wins/DocsWin.vue';
import NewsWin from './wins/NewsWin.vue';

export default function setup() {
  const tab = (id: string, g: string, order: number, component: typeof General, hidden?: () => boolean) => settingsTab({ id, label: tl(`config.tabs.${id}`), g, order, component, hidden });
  // The desktop app has one person at its own computer and nobody to sign in, a share link would lead to 127.0.0.1, and
  // it brings nothing to mount a remote with.
  const serverOnly = () => host.desktop;
  // Allgemein and Aussehen open the settings, as in the other apps, and the account follows as the part that is everybody's own.
  tab('general', 'IconTabGeneral', 10, General);
  tab('look', 'IconTabLook', 20, Look);
  tab('security', 'IconTabSecurity', 30, Security, serverOnly);
  tab('files', 'IconFiles', 40, FileList);
  tab('sources', 'IconTabStorage', 50, Sources);
  tab('remotes', 'IconCloud', 60, Remotes, serverOnly);
  tab('users', 'IconAccounts', 70, Users, serverOnly);
  tab('rules', 'IconShieldOn', 80, Rules, serverOnly);
  tab('shares', 'IconLink', 90, Shares, serverOnly);
  tab('integrations', 'IconModules', 100, Integrations);
  tab('app', 'IconTabApp', 110, Apps);
  tab('system', 'IconTabSystem', 120, System);
  tab('info', 'IconInfo', 130, Info);

  registerView('activity', { page: true, component: ActivityPage });
  const again = takeAgainFromUrl();
  if (again) go('settings', again === 'mcp' ? 'integrations' : 'security');

  action('settings', { label: tl('config.tabs.title'), g: 'IconGear', run: () => go('settings') });
  action('about', { label: tl('config.actions.about'), g: 'IconInfo', run: () => go('settings', 'info') });
  action('mounts', { label: tl('config.tabs.remotes'), g: 'IconCloud', when: () => !host.desktop, run: () => go('settings', 'remotes') });
  action('openShares', { label: tl('config.tabs.shares'), g: 'IconLink', when: () => !host.desktop, run: () => go('settings', 'shares') });
  action('openActivity', { label: tl('config.actions.activity'), g: 'IconLog', run: () => go('activity') });
  action('docs', { label: tl('config.actions.docs'), g: 'IconManual', run: () => win({ title: c('actions.docs'), wide: true, body: DocsWin }) });
  action('whatsNew', { label: tl('config.actions.whatsNew'), g: 'IconWhatsNew', run: async () => {
    const version = (await admin.system().catch(() => null))?.version ?? '';
    win({ title: c('news.title', { v: version }), wide: true, body: NewsWin, props: { version } });
  } });
}
