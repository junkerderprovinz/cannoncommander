import { describe, expect, it, vi } from 'vitest';
import { emit, on } from './sse';

describe('event stream', () => {
  it('hands an event of the desktop app only to those listening for its type', () => {
    const drop = vi.fn(), open = vi.fn();
    on('desktop.drop', drop);
    on('desktop.open', open);
    emit({ type: 'desktop.drop', data: { paths: ['C:\\a.txt'] } });
    expect(drop).toHaveBeenCalledWith({ type: 'desktop.drop', data: { paths: ['C:\\a.txt'] } });
    expect(open).not.toHaveBeenCalled();
  });
});
