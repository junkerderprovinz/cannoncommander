// A trip to a sign-in service from the tile, such as a test sign-in, comes back to the app's start with its outcome in
// the address. The settings open on the tile, which says it once.
import { takeSignInCode } from '@glimstone/core';

let arrived: string | null = null;

export function takeArrivedFromUrl(): boolean {
  arrived = takeSignInCode();
  return arrived !== null;
}

export function takeArrived(): string | null {
  const code = arrived;
  arrived = null;
  return code;
}
