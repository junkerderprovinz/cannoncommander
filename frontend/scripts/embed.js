// Copies the built interface into the folder the Go server embeds, without shell tools so it runs the same on Windows.
// The folder's .gitignore stays, because it is what keeps the folder in git for go:embed on a fresh checkout.
import { cpSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';

const dist = join(import.meta.dirname, '..', 'dist');
const embed = join(import.meta.dirname, '..', '..', 'backend', 'internal', 'web', 'embed');
for (const name of readdirSync(embed)) if (name !== '.gitignore') rmSync(join(embed, name), { recursive: true, force: true });
cpSync(dist, embed, { recursive: true });
console.log(`copied ${dist} to ${embed}`);
