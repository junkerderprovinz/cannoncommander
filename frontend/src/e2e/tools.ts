// The prototype's saved searches under the names the step files use: CC.smart.rows() hands back the rows of the search
// field's menu as markup, the way d-10-smart.js writes them.
import type { SearchQuery } from '@/api/tools';
import { saveFilter, saveSearch } from '@/features/search/saved';
import { usePanels } from '@/stores/panels';
import { useSearch } from '@/stores/search';

const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);

export function provideTools(provide: (name: string, value: unknown) => void) {
  const search = useSearch(), F = usePanels();
  provide('CC.smart', {
    save: (q: SearchQuery) => saveSearch(q),
    saveFilter,
    rows: () => search.saved.map((s) => `<button class="qrow ${F.T().smart === s.id ? 'cur' : ''}" data-smart="${s.id}"><span>${esc(s.name)}</span><span class="glim-num">${search.hits.get(s.id)?.total ?? ''}</span></button>`).join(''),
  });
}
