// jsdom lays nothing out and leaves out the calls that scroll, which pages make
// to keep the picked row or tile in sight. The tests that run in node have no
// elements at all.
if (typeof Element !== 'undefined') {
  Element.prototype.scrollIntoView ??= () => {};
  Element.prototype.scrollTo ??= () => {};
}

// GlimStone's selectors, README buttons and path fields measure themselves
// through a ResizeObserver, which jsdom does not have.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
