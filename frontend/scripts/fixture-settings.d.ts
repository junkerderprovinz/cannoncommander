// The settings pages, the account and the labels of the fixture, plain JavaScript for Vite's server like the rest of it.
import type { Connect } from 'vite';

export function settingsFixtureMiddleware(): Connect.NextHandleFunction & { terminalOn: () => boolean };
