// The jobs, trash, undo, upload and Unraid part of the fixture, plain JavaScript for Vite's server like the rest of it.
import type { Connect } from 'vite';

// A plugin gets the tree and the engine's helpers and hands back routes and job kinds of its own.
export type FixturePlugin = (engine: Record<string, unknown>) => { routes?: unknown[]; kinds?: Record<string, unknown> };
export function opsFixture(fx?: ReturnType<typeof import('./fixture.js').sharedFixture>, plugins?: FixturePlugin[]): Connect.NextHandleFunction;
export function opsFixtureMiddleware(): Connect.NextHandleFunction;
