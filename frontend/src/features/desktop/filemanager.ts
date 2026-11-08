// The switch that makes the desktop app the computer's file manager. While it is on, every start writes the entry again,
// so it follows the program to a new place after an update and speaks the language the window speaks.
import { reactive, watch } from 'vue';
import { call, json } from '@/api/client';
import { toast } from '@/core/toast';
import { language, t } from '@/i18n';

export const fileManager = reactive({ offered: false, on: false });

const words = () => ({ open: t('desktop.fmOpen'), show: t('desktop.fmShow'), quit: t('desktop.fmQuit') });

export async function loadFileManager() {
  try {
    const got = await call<{ on: boolean }>('/filemanager');
    Object.assign(fileManager, { offered: true, on: !!got.body?.on });
  } catch {
    Object.assign(fileManager, { offered: false, on: false });
  }
}

export async function setFileManager(on: boolean, quiet = false) {
  try {
    const got = await call<{ on: boolean }>('/filemanager', json('PUT', { on, words: words() }));
    fileManager.on = !!got.body?.on;
    if (!quiet) toast(t(fileManager.on ? 'desktop.fmNowOn' : 'desktop.fmNowOff'));
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'warn');
  }
}

export async function followFileManager() {
  await loadFileManager();
  if (fileManager.on) await setFileManager(true, true);
  watch(language, () => { if (fileManager.on) void setFileManager(true, true); });
}
