// Opening a file the way a double click in Explorer or the Linux file manager does: a document in its program, a
// program by running it. On Linux a program asks first, as the file managers there do.
import { ApiError, call, json } from '@/api/client';
import { run } from '@/core/actions';
import { toast } from '@/core/toast';
import { cancelBtn, win } from '@/core/windows';
import { esc, glyphSvg } from '@/features/viewer/markdown';
import { t } from '@/i18n';

export async function launch(path: string, start = false) {
  try {
    await call('/launch', json('POST', { path, run: start }));
  } catch (e) {
    if (e instanceof ApiError && e.problem?.code === 'askRun') return askRun(path);
    toast(e instanceof Error ? e.message : String(e), 'warn');
  }
}

function askRun(path: string) {
  const name = path.split('/').pop()!;
  win({
    title: t('desktop.runTitle'),
    body: `<div class="glim-subcard"><div class="dk-ask">${glyphSvg('IconFileProgram')}<span>${t('desktop.runText', { name: `<b>${esc(name)}</b>` })}</span></div></div>`,
    foot: [
      cancelBtn(),
      { label: t('desktop.show'), g: 'IconEye', run: () => { run('view'); } },
      { label: t('desktop.run'), g: 'IconPlay', primary: true, run: () => { void launch(path, true); } },
    ],
  });
}
