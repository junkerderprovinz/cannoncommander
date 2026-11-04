// In ArrowLoop planen: recurring runs belong to ArrowLoop, which already plans and logs them; this only hands a pair of
// folders over. The sync window opens it with run('arrowLoop', left, right, { direction, mirror }).
import { reactive } from 'vue';
import { ApiError } from '@/api/client';
import { type ArrowLoopJob, tools } from '@/api/tools';
import { action } from '@/core/actions';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { split } from '@/files/paths';
import { t, tl } from '@/i18n';
import { usePanels } from '@/stores/panels';
import PlanBody, { type PlanState } from './PlanBody.vue';

type Way = Pick<ArrowLoopJob, 'direction' | 'mirror'>;

function planWindow(left?: string, right?: string, way: Way = { direction: 'right' }) {
  const F = usePanels();
  const L = left ?? F.T(0).path, R = right ?? F.T(1).path;
  const st: PlanState = reactive({ name: t('arrowloop.to', { left: split(L).pop() || '/', right: split(R).pop() || '/' }), every: 'daily', at: '03:00' });
  win({
    title: t('arrowloop.title'), body: PlanBody, props: { st },
    foot: [cancelBtn(), { label: t('arrowloop.hand'), g: 'IconSchedules', primary: true, run: () => {
      const at = st.at.trim();
      if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(at)) { toast(t('arrowloop.badTime'), 'warn'); return false; }
      const name = st.name.trim() || t('arrowloop.fallback');
      tools.arrowLoop({ name, left: L, right: R, ...way, every: st.every, at }).then(({ url }) => {
        toast(t('arrowloop.planned', { name }), 'ok', { label: t('arrowloop.open'), run: () => { window.open(url, '_blank', 'noopener'); } });
      }, (e) => {
        const status = e instanceof ApiError ? e.status : 0;
        toast(status === 409 ? t('arrowloop.taken') : status === 503 ? t('arrowloop.notSet') : t('arrowloop.noAnswer', { why: e.message }), 'warn');
      });
      return true;
    } }],
  });
}

export default function setup() {
  action('arrowLoop', { label: tl('arrowloop.title'), g: 'IconSchedules', needs: 'dual', run: planWindow });
}
