// Die Sammelmappe: entries from several folders wait here until they go out under one link. It lives in this tab only.
import { reactive } from 'vue';

export interface Collected { path: string; name: string; dir: boolean; size: number }
export const basket = reactive<Collected[]>([]);

// The other side sees one folder; two entries of the same name from different folders get a number.
export function asFolder(items: Collected[]) {
  const seen: Record<string, number> = {};
  return items.map((x) => {
    const n = (seen[x.name] = (seen[x.name] || 0) + 1), dot = x.dir ? -1 : x.name.lastIndexOf('.');
    return { ...x, shown: n === 1 ? x.name : dot > 0 ? `${x.name.slice(0, dot)} (${n})${x.name.slice(dot)}` : `${x.name} (${n})` };
  });
}
