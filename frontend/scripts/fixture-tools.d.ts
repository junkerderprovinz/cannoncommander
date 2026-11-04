// The search and tools part of the fixture, a plugin for the job engine of fixture-ops.js.
import type { FixturePlugin } from './fixture-ops.js';

export function diffLines(a: string[], b: string[]): { t: 'eq' | 'chg' | 'del' | 'add'; l: number | null; r: number | null }[];
export const toolsFixture: FixturePlugin;
