// The remotes as everybody sees them in the sidebar: where each is mounted and whether it is. The settings tell this
// list after every change they make, so the sidebar follows without asking the server on its own.
import { reactive } from 'vue';
import { admin, type Remote } from '@/api/admin';

export const remotes = reactive({ list: [] as Remote[] });

export async function reloadRemotes() {
  try {
    remotes.list = (await admin.remotes()).remotes;
  } catch {
    // Without an answer the sidebar keeps what it showed.
  }
}
