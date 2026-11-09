// Mounted remotes in the sidebar of the container, each one click away from the shortcuts. The desktop app mounts
// nothing, so it shows none.
import './remotes.css';
import { h, watch } from 'vue';
import { on } from '@/api/sse';
import { addTo } from '@/core/extend';
import { host } from '@/core/host';
import RemotesCard from './RemotesCard.vue';
import { reloadRemotes } from './store';

export default function setup() {
  addTo('sideCards', () => h(RemotesCard));
  on('resync', () => { if (!host.desktop) void reloadRemotes(); });
  watch(() => host.desktop, (desktop) => { if (!desktop) void reloadRemotes(); }, { immediate: true });
}
