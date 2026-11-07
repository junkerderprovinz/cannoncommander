import { describe, expect, it } from 'vitest';
import file from '@/assets/brand/cannoncommander-kopfzeile.svg?raw';
import { HEAD_LOGO, split } from './logo';

const doc = new DOMParser().parseFromString(file, 'image/svg+xml');
const paths = (sel: string) => [...doc.querySelectorAll(sel)].map((p) => p.getAttribute('d') ?? `${p.getAttribute('cx')},${p.getAttribute('cy')}`);
const shapes = (markup: string) => [...new DOMParser().parseFromString(`<svg xmlns="http://www.w3.org/2000/svg">${markup}</svg>`, 'image/svg+xml').querySelectorAll('path, circle')];
const drawn = (markup: string) => shapes(markup).map((p) => p.getAttribute('d') ?? `${p.getAttribute('cx')},${p.getAttribute('cy')}`);

describe('the header logo', () => {
  it('keeps the viewBox and the proportions of the file', () => {
    expect(HEAD_LOGO.viewBox).toBe('0 0 1000 231.67');
    expect(HEAD_LOGO.ratio).toBeCloseTo(1000 / 231.67);
  });

  it('flies exactly the shapes of the ball in the file', () => {
    expect(drawn(HEAD_LOGO.ball)).toEqual(paths('#bombe path, #bombe circle'));
  });

  it('leaves the ball out of the cannon and draws everything else', () => {
    const ball = new Set(paths('#bombe path, #bombe circle'));
    const rest = paths('#kannonen path, #kannonen circle').filter((d) => !ball.has(d));
    expect(drawn(HEAD_LOGO.cannon)).toEqual(rest);
  });

  it('carries the colours of the file on the shapes, so no class of it reaches the page', () => {
    const fills = new Map([...file.matchAll(/\.(cls-\d+)\s*\{\s*fill:\s*(#[0-9a-f]+)/gi)].map((m) => [m[1]!, m[2]!]));
    const want = [...doc.querySelectorAll('#bombe path, #bombe circle')].map((p) => fills.get(p.getAttribute('class')!));
    expect(shapes(HEAD_LOGO.ball).map((p) => p.getAttribute('fill'))).toEqual(want);
    expect(HEAD_LOGO.cannon + HEAD_LOGO.ball).not.toMatch(/class=|id=/);
  });

  it('finds the ball by its group, whatever the drawing around it', () => {
    const tiny = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 4"><style>.a{fill:#123456}</style><g id="kannonen"><rect class="a" width="5" height="2"/><g id="bombe"><circle class="a" cx="8" cy="2" r="1"/></g></g></svg>';
    const s = split(tiny);
    expect(s.viewBox).toBe('0 0 10 4');
    expect(s.ball).toContain('<circle');
    expect(s.ball).toContain('fill="#123456"');
    expect(s.cannon).toContain('<rect');
    expect(s.cannon).not.toContain('<circle');
  });
});
