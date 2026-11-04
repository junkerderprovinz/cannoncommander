// The jobs, trash, undo and upload part of the fixture, plain JavaScript for Vite's server like the rest of it.
import type { Connect } from 'vite';

export function opsFixture(): Connect.NextHandleFunction;
export function opsFixtureMiddleware(): Connect.NextHandleFunction;
// The queue and tree helpers the other parts of the fixture run their jobs with.
export function sharedOps(): Record<string, unknown>;
