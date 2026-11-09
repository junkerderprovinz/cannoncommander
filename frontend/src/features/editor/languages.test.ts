import { describe, expect, it } from 'vitest';
import { detect, LANGUAGES, shebang } from './languages';

const id = (name: string, first?: string) => detect(name, first)?.id;

describe('languages', () => {
  it('knows a file by its extension, in any case', () => {
    expect(id('main.go')).toBe('go');
    expect(id('App.TSX')).toBe('tsx');
    expect(id('docker-compose.yml')).toBe('yaml');
    expect(id('style.scss')).toBe('scss');
    expect(id('Program.cs')).toBe('csharp');
    expect(id('nginx.conf')).toBe('nginx');
    expect(id('unknown.xyz')).toBeUndefined();
  });

  it('knows files by their whole name', () => {
    expect(id('Dockerfile')).toBe('dockerfile');
    expect(id('Dockerfile.dev')).toBe('dockerfile');
    expect(id('Makefile')).toBe('makefile');
    expect(id('CMakeLists.txt')).toBe('cmake');
    expect(id('.bashrc')).toBe('shell');
    expect(id('go')).toBe('shell');
  });

  it('reads the program from a shebang', () => {
    expect(shebang('#!/bin/bash')).toBe('bash');
    expect(shebang('#!/usr/bin/env python3.11')).toBe('python');
    expect(shebang('#!/usr/bin/env -S node --no-warnings')).toBe('node');
    expect(shebang('#! /usr/bin/perl -w')).toBe('perl');
    expect(shebang('echo hi')).toBe('');
    expect(id('backup', '#!/bin/sh\nrsync')).toBe('shell');
    expect(id('serve', '#!/usr/bin/env python3')).toBe('python');
    expect(id('page', '<!DOCTYPE html>')).toBe('html');
    expect(id('feed', '<?xml version="1.0"?>')).toBe('xml');
  });

  it('leaves a text file plain even when it starts like a script', () => {
    expect(id('notes.txt', '#!/bin/bash')).toBeUndefined();
  });

  it('loads every language', async () => {
    for (const l of LANGUAGES) expect(await l.load(), l.id).toBeTruthy();
  });

  it('gives every language a unique id and every extension one language', () => {
    expect(new Set(LANGUAGES.map((l) => l.id)).size).toBe(LANGUAGES.length);
    const exts = LANGUAGES.flatMap((l) => l.ext ?? []);
    expect(exts.filter((e, i) => exts.indexOf(e) !== i)).toEqual([]);
  });
});
