import { describe, expect, it } from 'vitest';
import { withoutLayers } from './layers';

describe('a stylesheet without layers', () => {
  it('keeps the layered rules, nested ones too, ahead of the unlayered ones', () => {
    const css = '@layer base, components;\n.own { color: red; }\n@layer components { .a { display: none; } @media (min-width: 1px) { .b { order: 1; } } }\n@layer base { @layer inner { .c { gap: 0; } } }';
    const flat = withoutLayers(css);
    expect(flat).not.toContain('@layer');
    expect(flat.indexOf('.a {')).toBeLessThan(flat.indexOf('.own'));
    expect(flat.indexOf('.c {')).toBeLessThan(flat.indexOf('.own'));
    expect(flat).toContain('@media (min-width: 1px) { .b { order: 1; } }');
  });

  it('lets jsdom apply a rule that stood in a layer', () => {
    const style = document.head.appendChild(Object.assign(document.createElement('style'), { textContent: withoutLayers('@layer components { .gone { display: none; } }') }));
    const el = document.body.appendChild(Object.assign(document.createElement('i'), { className: 'gone' }));
    try {
      expect(getComputedStyle(el).display).toBe('none');
    } finally {
      style.remove();
      el.remove();
    }
  });
});
