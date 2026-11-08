// jsdom lays nothing out and leaves out the calls that scroll, which pages make
// to keep the picked row or tile in sight. The tests that run in node have no
// elements at all.
if (typeof Element !== 'undefined') {
  Element.prototype.scrollIntoView ??= () => {};
  Element.prototype.scrollTo ??= () => {};
}
