// Parts that come in chunks of their own. A chunk can fail to arrive while the page navigates or the network drops for
// a moment, and a second try usually lands, so every load tries twice. A part that still does not arrive says so where
// it would stand and offers to try again, instead of leaving an error nobody catches.
import { Button } from '@glimstone/vue';
import { type Component, defineComponent, h, ref, shallowRef } from 'vue';
import { t } from '@/i18n';

export const twice = <T>(load: () => Promise<T>) => () => load().catch(() => load());

const LoadFailed = defineComponent({
  emits: ['retry'],
  setup(_, { emit }) {
    return () => h('div', { class: 'lazy-failed', role: 'status' }, [
      h('p', t('core.loadFailed')),
      h(Button, { label: t('core.retry'), labelKey: 'IconRetry', 'data-lazy': 'retry', onClick: () => emit('retry') }),
    ]);
  },
});

// Hands its props, listeners and slots on to the part once it has arrived.
export function lazyComponent(load: () => Promise<{ default: Component }>): Component {
  let arrived: Component | null = null;
  return defineComponent({
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      const part = shallowRef<Component | null>(arrived);
      const failed = ref(false);
      const fetch = () => {
        failed.value = false;
        twice(load)().then((m) => { part.value = arrived = m.default; }, () => { failed.value = true; });
      };
      if (!arrived) fetch();
      return () => (part.value ? h(part.value, attrs, slots) : failed.value ? h(LoadFailed, { onRetry: fetch }) : null);
    },
  });
}
