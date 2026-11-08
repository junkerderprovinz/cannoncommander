// The settings tabs, the activity page and the help actions that lead into them.
import './config.css';
import { action } from '@/core/actions';
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
  const tab = (id: string, g: string, order: number, component: typeof General) => settingsTab({ id, label: tl(`config.tabs.${id}`), g, order, component });
  // Allgemein and Aussehen open the settings, as in the other apps, and the account follows as the part that is everybody's own.
  tab('general', 'IconTabGeneral', 10, General);
  tab('look', 'IconTabLook', 20, Look);
  tab('security', 'IconTabSecurity', 30, Security);
  tab('files', 'IconFiles', 40, FileList);
  tab('sources', 'IconTabStorage', 50, Sources);
  tab('remotes', 'IconCloud', 60, Remotes);
  tab('users', 'IconAccounts', 70, Users);
  tab('rules', 'IconShieldOn', 80, Rules);
  tab('shares', 'IconLink', 90, Shares);
  tab('integrations', 'IconModules', 100, Integrations);
  tab('app', 'IconTabApp', 110, Apps);
  tab('system', 'IconTabSystem', 120, System);
  tab('info', 'IconInfo', 130, Info);

  registerView('activity', { page: true, component: ActivityPage });
  if (takeAgainFromUrl()) go('settings', 'security');

  action('settings', { label: tl('config.tabs.title'), g: 'IconGear', run: () => go('settings') });
  action('about', { label: tl('config.actions.about'), g: 'IconInfo', run: () => go('settings', 'info') });
  action('mounts', { label: tl('config.tabs.remotes'), g: 'IconCloud', run: () => go('settings', 'remotes') });
  action('openShares', { label: tl('config.tabs.shares'), g: 'IconLink', run: () => go('settings', 'shares') });
  action('openActivity', { label: tl('config.actions.activity'), g: 'IconLog', run: () => go('activity') });
  action('docs', { label: tl('config.actions.docs'), g: 'IconManual', run: () => win({ title: c('actions.docs'), wide: true, body: DocsWin }) });
  action('whatsNew', { label: tl('config.actions.whatsNew'), g: 'IconWhatsNew', run: async () => {
    const version = (await admin.system().catch(() => null))?.version ?? '';
    win({ title: c('news.title', { v: version }), wide: true, body: NewsWin, props: { version } });
  } });
}
