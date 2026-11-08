// What the desktop app knows of the computer: the folders every user has, such as Downloads, and the drives, which
// come and go while it runs.
import { reactive, ref } from 'vue';
import { call, json } from '@/api/client';
import type { DriveKind } from '@/api/types';

export type PlaceKind = 'home' | 'desktop' | 'documents' | 'downloads' | 'pictures' | 'music' | 'videos';
export interface Place { kind: PlaceKind; path: string }
// path is missing while the volume waits to be mounted, as Linux leaves a partition until someone opens it.
export interface Volume { id: string; path?: string; label?: string; kind: DriveKind; remote?: string; size?: number; eject?: boolean }
export interface VolumesEvent { added: Volume[]; removed: Volume[] }

export const places = reactive({ folders: [] as Place[], volumes: [] as Volume[] });
// Counts the folders opened, since the list of those used last lives outside Vue.
export const visited = ref(0);

export async function loadPlaces() {
  const { body } = await call<{ folders: Place[]; volumes: Volume[] }>('/places');
  if (body) Object.assign(places, body);
}

export const within = (path: string, v: Volume) => !!v.path && (path === v.path || path.startsWith(v.path + '/') || path.startsWith(v.path + '::'));

// Where a volume is, mounting it first when it waits for that. It is asked by id, since a volume the desktop just
// mounted by itself has its folder by now.
export async function reachVolume(v: Volume): Promise<string> {
  const now = places.volumes.find((o) => o.id === v.id) ?? v;
  if (now.path) return now.path;
  const { body } = await call<{ path: string }>('/places/mount', json('POST', { id: v.id }));
  return body!.path;
}

export const ejectVolume = (v: Volume) => call('/places/eject', json('POST', { id: v.id }));
