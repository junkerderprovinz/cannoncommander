import { describe, expect, it } from 'vitest';
import { crumbs } from './crumbs';

// The share the GNOME pass opened in Nautilus, as gvfs mounted it for user 1000.
const share = '/run/user/1000/gvfs/smb-share:server=localhost,share=ccshare';

describe('the parts of the path bar', () => {
  it('are the folders of the path, each with the path it opens', () => {
    expect(crumbs('/home/tester/Bilder')).toEqual([
      { path: '/home', part: 'home' },
      { path: '/home/tester', part: 'tester' },
      { path: '/home/tester/Bilder', part: 'Bilder' },
    ]);
    expect(crumbs('/')).toEqual([]);
  });

  it('start a network share mounted deep down at the network and the share, as Nautilus does', () => {
    expect(crumbs(share + '/Fotos/2024', share)).toEqual([
      { path: '/run/user/1000/gvfs', part: '', network: true },
      { path: share, part: 'smb-share:server=localhost,share=ccshare' },
      { path: share + '/Fotos', part: 'Fotos' },
      { path: share + '/Fotos/2024', part: '2024' },
    ]);
    expect(crumbs(share, share)).toEqual([
      { path: '/run/user/1000/gvfs', part: '', network: true },
      { path: share, part: 'smb-share:server=localhost,share=ccshare' },
    ]);
  });

  it('leave a path outside the share as it is', () => {
    expect(crumbs('/run/user/1000', share)).toHaveLength(3);
  });
});
