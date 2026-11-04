// The prototype's CC.unraid and CC.parity for the step files, read from the Unraid store. The step files ask for
// the disk of a file synchronously, as the prototype keeps it on the node, so a disk not in the listing yet is
// fetched with a synchronous request; this only ever runs in the test build.
import { BASE } from '@/api/client';
import { useUnraid } from '@/features/unraid/state';
import { useListing } from '@/stores/listing';

interface Node { name: string; dir?: boolean; link?: string; children?: Node[] }

// fresh asks again for paths already known, as a move can have put a file on another disk since.
export function devicesNow(paths: string[], fresh = false): void {
  const listing = useListing();
  const missing = paths.filter((p) => fresh || listing.meta.get(p)?.disk === undefined);
  if (!missing.length) return;
  const req = new XMLHttpRequest();
  req.open('POST', BASE + '/fs/meta', false);
  req.setRequestHeader('Content-Type', 'application/json');
  req.send(JSON.stringify({ paths: missing, columns: ['disk'] }));
  if (req.status !== 200) return;
  const got = JSON.parse(req.responseText).items as Record<string, Record<string, string | null>>;
  for (const [p, values] of Object.entries(got)) listing.meta.set(p, { ...listing.meta.get(p), ...values });
}

export function devOf(path: string): string | undefined {
  devicesNow([path], true);
  const v = useListing().meta.get(path)?.disk;
  return typeof v === 'string' && v ? v.split(', ')[0] : undefined;
}

export function provideUnraid(provide: (name: string, value: unknown) => void, pathOf: (n: Node) => string | undefined) {
  const U = useUnraid();
  const filesOf = (node: Node | null): [Node & { dev?: string }, string][] => {
    if (!node) return [];
    const out: [Node, string][] = [];
    const walk = (n: Node, rel: string) => {
      if (!n.dir || n.link) out.push([n, rel]);
      else for (const k of n.children || []) walk(k, rel + '/' + k.name);
    };
    walk(node, node.name);
    devicesNow(out.map(([n]) => pathOf(n)).filter((p): p is string => !!p));
    return out;
  };
  provide('CC.unraid', {
    get DISKS() { return U.disks.map((d) => d.name); },
    get spin() { return Object.fromEntries(U.disks.map((d) => [d.name, !d.spundown])); },
    filesOf,
  });
  const fixture = (busy: boolean, finished = false) => fetch(BASE + '/fixture/parity', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ busy, finished }) });
  provide('CC.parity', {
    get running() { return !!U.st?.parity.running; },
    get pct() { return Math.floor(U.st?.parity.percent ?? 0); },
    start: () => fixture(true),
    stop: (finished: boolean) => fixture(false, finished),
  });
}
