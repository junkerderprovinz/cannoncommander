// Fenster-Profile keep the layout of both panels on the server, and Auswahl merken keeps a selection to bring back
// later, for instance after a copy cleared it, as Krusader does.
import { reactive } from 'vue';
import { type Profile, tools } from '@/api/tools';
import { action } from '@/core/actions';
import { prefs, setPref } from '@/core/prefs';
import { toast } from '@/core/toast';
import { win } from '@/core/windows';
import { Columns } from '@/files/columns';
import { plural, t, tl } from '@/i18n';
import { newTab, usePanels } from '@/stores/panels';
import ProfilesBody from './ProfilesBody.vue';

const warn = (e: Error) => toast(e.message, 'warn');

export default function setup() {
  const F = usePanels();

  let saved: { path: string; names: string[] } | null = null;
  action('saveSel', { label: tl('profiles.saveSel'), g: 'IconPin', needs: 'sel', run: () => {
    saved = { path: F.T().path, names: F.sel().map((k) => k.name) };
    toast(plural(saved.names.length, 'profiles.selSaved'));
  } });
  action('restoreSel', { label: tl('profiles.restoreSel'), g: 'IconRestore', when: () => !!saved, run: async () => {
    const s = saved!;
    if (F.T().path !== s.path) await F.go(s.path);
    const here = new Set(F.entries().map((k) => k.name));
    F.T().marked = new Set(s.names.filter((n) => here.has(n)));
  } });

  action('profiles', { label: tl('profiles.title'), g: 'IconDashboard', run: () => {
    const st = reactive({ list: [] as Profile[], name: '' });
    tools.profiles().then((list) => { st.list = list; }, warn);
    const load = (p: Profile) => {
      p.panels.forEach((x, i) => Object.assign(F.P(i), { cur: x.cur, tabs: x.tabs.map(newTab) }));
      setPref('dual', p.dual);
      w.close();
      Columns.set(p.columns);
      toast(t('profiles.loaded', { name: p.name }));
    };
    const drop = (p: Profile) => tools.deleteProfile(p.id).then(() => { st.list = st.list.filter((x) => x !== p); }, warn);
    const w = win({
      title: t('profiles.title'), body: ProfilesBody, props: { st, load, drop },
      foot: [{ label: t('profiles.save'), g: 'IconSave', primary: true, run: () => {
        const name = st.name.trim();
        if (!name) { toast(t('profiles.nameFirst'), 'warn'); return false; }
        const panels = F.panels.map((p) => ({ cur: p.cur, tabs: p.tabs.map((tab) => tab.path) }));
        tools.saveProfile({ name, columns: Columns.shown(), dual: !!prefs.dual, panels }).then(() => toast(t('profiles.savedOne', { name })), warn);
        return true;
      } }],
    });
  } });
}
