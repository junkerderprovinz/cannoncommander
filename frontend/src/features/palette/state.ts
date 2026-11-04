import { reactive } from 'vue';
import { run } from '@/core/actions';
import { toast } from '@/core/toast';
import { type Line, remember } from './rows';

// focus counts the times Strg+K came while the search was open, which hands the field the focus again.
export const pal = reactive({ open: false, focus: 0 });

export function pick(r: Line | undefined) {
  if (!r || 'head' in r) return;
  if (r.why) return toast(r.why, 'warn');
  pal.open = false;
  if (r.go) return r.go();
  remember(r.a!.id);
  run(r.a!.id);
}
