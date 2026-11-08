// jsdom lays nothing out and leaves out the calls that scroll, which pages make
// to keep the picked row or tile in sight. Every test runs with them as no-ops.
Element.prototype.scrollIntoView ??= () => {};
Element.prototype.scrollTo ??= () => {};
