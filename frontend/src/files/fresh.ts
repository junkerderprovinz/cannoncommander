import { reactive } from 'vue';

// Paths that changed on the server a moment ago; their rows glow until the moment passes.
export const fresh = reactive(new Set<string>());
