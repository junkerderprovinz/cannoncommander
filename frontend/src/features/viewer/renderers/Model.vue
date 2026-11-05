<script setup lang="ts">
// A 3D model on a turntable: dragging turns it, the wheel brings it closer.
import { Pill } from '@glimstone/vue';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { t } from '@/i18n';
import { rawUrl } from '../content';
import { ext, joinPath } from '../kinds';
import type { REmits, RProps } from './types';

const props = defineProps<RProps>();
const emit = defineEmits<REmits & { (e: 'failed'): void }>();
const host = ref<HTMLElement | null>(null);
let stop = () => {};

onMounted(async () => {
  try {
    const THREE = await import('three');
    const { OrbitControls } = await import('three/addons/controls/OrbitControls.js');
    const e = ext(props.n.name);
    const res = await fetch(rawUrl(joinPath(props.folder, props.n.name)));
    if (!res.ok) throw new Error(String(res.status));
    const data = await res.arrayBuffer();
    let object: import('three').Object3D;
    if (e === 'stl' || e === 'ply') {
      const Loader = e === 'stl' ? (await import('three/addons/loaders/STLLoader.js')).STLLoader : (await import('three/addons/loaders/PLYLoader.js')).PLYLoader;
      const geometry = new Loader().parse(data);
      geometry.computeVertexNormals();
      object = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color: 0xb8bcc4, metalness: 0.1, roughness: 0.6 }));
    } else if (e === 'obj') {
      object = new (await import('three/addons/loaders/OBJLoader.js')).OBJLoader().parse(new TextDecoder().decode(data));
    } else if (e === '3mf') {
      object = new (await import('three/addons/loaders/3MFLoader.js')).ThreeMFLoader().parse(data);
    } else if (e === 'fbx') {
      object = new (await import('three/addons/loaders/FBXLoader.js')).FBXLoader().parse(data, '');
    } else {
      const { GLTFLoader } = await import('three/addons/loaders/GLTFLoader.js');
      object = (await new GLTFLoader().parseAsync(data, '')).scene;
    }

    const el = host.value!;
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setPixelRatio(devicePixelRatio);
    el.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    scene.add(new THREE.HemisphereLight(0xffffff, 0x444444, 2.2), Object.assign(new THREE.DirectionalLight(0xffffff, 1.6), { position: new THREE.Vector3(3, 5, 4) }));
    const box = new THREE.Box3().setFromObject(object), size = box.getSize(new THREE.Vector3()).length() || 1;
    object.position.sub(box.getCenter(new THREE.Vector3()));
    scene.add(object);
    const camera = new THREE.PerspectiveCamera(40, 1, size / 100, size * 100);
    camera.position.set(size * 0.7, size * 0.5, size * 0.9);
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.enabled = !props.compact;
    controls.autoRotate = !!props.compact;
    const fit = () => {
      const w = el.clientWidth, h = el.clientHeight || w * 0.75;
      renderer.setSize(w, h);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    };
    const seen = new ResizeObserver(fit);
    seen.observe(el);
    renderer.setAnimationLoop(() => { controls.update(); renderer.render(scene, camera); });
    stop = () => { seen.disconnect(); renderer.setAnimationLoop(null); controls.dispose(); renderer.dispose(); };
    emit('info', t('viewer.model.info', { format: e.toUpperCase() }));
  } catch {
    emit('failed');
  }
});
onBeforeUnmount(() => stop());
</script>

<template>
  <div v-if="!compact" class="cv-bar"><Pill>{{ t('viewer.model.hint') }}</Pill></div>
  <div ref="host" class="cv-model" :class="{ compact }"></div>
</template>
