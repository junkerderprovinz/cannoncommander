// One renderer per kind of file. The heavy ones load the first time a file of their kind is opened.
import type { Component } from 'vue';
import { lazyComponent } from '@/core/lazy';
import type { ViewKind } from '../kinds';
import Arch from './Arch.vue';
import Audio from './Audio.vue';
import Dir from './Dir.vue';
import Disk from './Disk.vue';
import Hex from './Hex.vue';
import Img from './Img.vue';
import Md from './Md.vue';
import Office from './Office.vue';
import Pages from './Pages.vue';
import Text from './Text.vue';
import Video from './Video.vue';

export const RENDERERS: Record<ViewKind, Component> = {
  dir: Dir, img: Img, video: Video, audio: Audio, pdf: Pages, office: Office, md: Md, text: Text, code: Text,
  arch: Arch, disk: Disk, file: Hex,
  epub: lazyComponent(() => import('./Epub.vue')),
  model: lazyComponent(() => import('./Model.vue')),
};
