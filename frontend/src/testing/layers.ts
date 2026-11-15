// jsdom drops every rule inside an @layer block, and GlimStone keeps its components in one. Lifted out and put ahead
// of the stylesheet's unlayered rules, they lose to those as they would in a browser, with source order standing in
// for the layer order.
export function withoutLayers(css: string): string {
  const layered: string[] = [];
  let rest = '';
  let at = 0;
  const layer = /@layer[^{;]*([{;])/g;
  for (let m = layer.exec(css); m; m = layer.exec(css)) {
    rest += css.slice(at, m.index);
    at = layer.lastIndex;
    if (m[1] === ';') continue;
    const start = at;
    for (let depth = 1; depth > 0; at++) {
      if (css[at] === '{') depth++;
      else if (css[at] === '}') depth--;
    }
    layered.push(withoutLayers(css.slice(start, at - 1)));
    layer.lastIndex = at;
  }
  return [...layered, rest + css.slice(at)].join('\n');
}
