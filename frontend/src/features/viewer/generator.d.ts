// The test tree's generator, which the office reader tests borrow real Office files from.
declare module '*/testing/generate/formats.mjs' {
  interface Layout { length: number; chunks: { offset: number; buf: Uint8Array }[] }
  export function office(ext: string, o: { title: string; lines: string[]; rows?: (string | number)[][]; thumbnail?: Uint8Array | null; time: number; size: number }): Layout;
}
