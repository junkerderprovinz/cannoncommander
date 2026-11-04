// The saved searches and folder tools part of the fixture, plain JavaScript for Vite's server like the rest of it.
import type { Connect } from 'vite';

export function diffLines(a: string[], b: string[]): { t: 'eq' | 'chg' | 'del' | 'add'; l: number | null; r: number | null }[];
export function toolsFixture(fx?: ReturnType<typeof import("./fixture.js").sharedFixture>, ops?: Record<string, unknown>): Connect.NextHandleFunction;
export function toolsFixtureMiddleware(): Connect.NextHandleFunction;
