import { describe, expect, it } from 'vitest';
import { pathCommand } from './words';

const here = '/mnt/user/appdata';

describe('pathCommand', () => {
  it('takes every trigger word as a wish for a terminal, in any case', () => {
    for (const word of ['cmd', 'sh', 'bash', 'terminal', 'powershell', 'pwsh', 'wt', 'CMD', 'PowerShell', ' Bash ']) {
      expect(pathCommand(word, here, [])).toEqual({ terminal: true });
    }
  });

  it('leaves everything else to the path field', () => {
    for (const typed of ['', '/mnt/user', 'bashrc', 'cmd.exe', 'open terminal', '/bin/bash', './sh', 'bash /tmp']) {
      expect(pathCommand(typed, here, [])).toBeNull();
    }
  });

  it('opens a folder here that is named like the word instead', () => {
    const items = [{ name: 'bash', dir: true }, { name: 'cmd', dir: false }];
    expect(pathCommand('bash', here, items)).toEqual({ folder: '/mnt/user/appdata/bash' });
    expect(pathCommand('cmd', here, items)).toEqual({ terminal: true });
  });

  it('finds that folder whatever its case, the exact name first', () => {
    expect(pathCommand('terminal', here, [{ name: 'Terminal', dir: true }])).toEqual({ folder: '/mnt/user/appdata/Terminal' });
    expect(pathCommand('Wt', here, [{ name: 'WT', dir: true }, { name: 'Wt', dir: true }])).toEqual({ folder: '/mnt/user/appdata/Wt' });
  });

  it('builds the folder path at the top of the server too', () => {
    expect(pathCommand('sh', '/', [{ name: 'sh', dir: true }])).toEqual({ folder: '/sh' });
  });
});
