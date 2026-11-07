// The header logo is drawn inline rather than as an image, so the easter egg can shake the cannon and fire the ball
// that the file draws as a group of its own. The file's stylesheet would apply to the whole page once inline, so its
// colours move onto the shapes themselves.
import raw from '@/assets/brand/cannoncommander-kopfzeile.svg?raw';

export interface Logo { viewBox: string; ratio: number; cannon: string; ball: string }

export function split(svg: string): Logo {
  const doc = new DOMParser().parseFromString(svg, 'image/svg+xml');
  const root = doc.documentElement;
  const fills = new Map([...(root.querySelector('style')?.textContent ?? '').matchAll(/\.([\w-]+)\s*\{\s*fill:\s*([^;}\s]+)/g)].map((m) => [m[1]!, m[2]!]));
  for (const el of root.querySelectorAll('[class]')) {
    const fill = fills.get(el.getAttribute('class')!);
    if (fill) el.setAttribute('fill', fill);
    el.removeAttribute('class');
  }
  const ball = root.querySelector('#bombe')!;
  const cannon = ball.parentElement!;
  ball.remove();
  // Ids would repeat on the page once the ball flies as a copy of itself.
  const markup = (el: Element) => {
    for (const named of [el, ...el.querySelectorAll('[id]')]) named.removeAttribute('id');
    return new XMLSerializer().serializeToString(el).replace(/ xmlns="[^"]*"/g, '');
  };
  const viewBox = root.getAttribute('viewBox')!;
  const [, , w, h] = viewBox.split(/[\s,]+/).map(Number);
  return { viewBox, ratio: w! / h!, cannon: markup(cannon), ball: markup(ball) };
}

export const HEAD_LOGO = split(raw);
