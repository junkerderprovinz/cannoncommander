// The settings tabs, the activity page and the help actions that lead into them.
import './config.css';
import { action } from '@/core/actions';
import { registerView, settingsTab, go } from '@/core/views';
import { win } from '@/core/windows';
import { admin } from '@/api/admin';
import { tl } from '@/i18n';
import ActivityPage from './ActivityPage.vue';
import { c } from '@/features/config/shared';
import App from './tabs/App.vue';
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
import Users from './tabs/Users.vue';
import DocsWin from './wins/DocsWin.vue';
import NewsWin from './wins/NewsWin.vue';

export default function setup() {
  const tab = (id: string, g: string, order: number, component: typeof General) => settingsTab({ id, label: tl(`config.tabs.${id}`), g, order, component });
  tab('general', 'IconTabGeneral', 10, General);
  tab('look', 'IconTabLook', 20, Look);
  tab('files', 'IconFiles', 30, FileList);
  tab('sources', 'IconTabStorage', 40, Sources);
  tab('remotes', 'IconCloud', 50, Remotes);
  tab('users', 'IconAccounts', 60, Users);
  tab('rules', 'IconShieldOn', 70, Rules);
  tab('shares', 'IconLink', 80, Shares);
  tab('security', 'IconTabSecurity', 90, Security);
  tab('integrations', 'IconModules', 100, Integrations);
  tab('app', 'IconTabApp', 110, App);
  tab('info', 'IconInfo', 120, Info);

  registerView('activity', { page: true, component: ActivityPage });

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
