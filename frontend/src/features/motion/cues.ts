// The triggers of the little surprises, kept apart from what they draw so they can be tested without a page.

export const KONAMI = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];

// Compares the last keys pressed, so a sequence that starts inside a failed attempt still counts.
export function sequence(keys: string[]) {
  let got: string[] = [];
  return (key: string) => {
    got = [...got, key.length === 1 ? key.toLowerCase() : key].slice(-keys.length);
    if (got.length < keys.length || got.some((k, i) => k !== keys[i])) return false;
    got = [];
    return true;
  };
}

// Only a text that grows into the word counts, so deleting back onto it plays nothing twice.
export function typedWord(word: string) {
  return (before: string, after: string) => after.length > before.length && after.toLowerCase().endsWith(word);
}

export function onceEach() {
  const seen = new Set<string>();
  return (key: string) => !seen.has(key) && !!seen.add(key);
}

export const BROADSIDE = 100;
export const broadside = (level: string, count: number) => level === 'storm' && count >= BROADSIDE;
