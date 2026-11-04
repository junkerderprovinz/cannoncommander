// A live region for screen readers, saying what changed out of sight.
let live: HTMLDivElement | null = null;
let clear: ReturnType<typeof setTimeout> | undefined;

export function installAnnouncer() {
  live = document.createElement('div');
  live.id = 'cc-live';
  live.className = 'cc-sr';
  live.setAttribute('role', 'status');
  live.setAttribute('aria-live', 'polite');
  document.body.appendChild(live);
}

// The text goes in after a short pause, so the same sentence twice is read twice.
export function say(text: string) {
  if (!live) return;
  const el = live;
  el.textContent = '';
  clearTimeout(clear);
  setTimeout(() => {
    el.textContent = text;
    clear = setTimeout(() => { el.textContent = ''; }, 6000);
  }, 60);
}
