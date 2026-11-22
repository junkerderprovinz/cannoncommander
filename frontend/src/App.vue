<script setup lang="ts">
import { SignInReauthWindow } from '@glimstone/vue';
import { computed } from 'vue';
import MenuHost from '@/core/MenuHost.vue';
import { prefs } from '@/core/prefs';
import ToastHost from '@/core/ToastHost.vue';
import { currentView, view } from '@/core/views';
import WindowHost from '@/core/WindowHost.vue';
import BottomBar from '@/shell/BottomBar.vue';
import FKeys from '@/shell/FKeys.vue';
import Head from '@/shell/Head.vue';
import { shell } from '@/shell/state';
import { session } from '@/signin';
import { reauthClient } from '@/signin/kit';
import SignInPage from '@/signin/SignInPage.vue';
import { reauthText } from '@/signin/words';
import { useUser } from '@/stores/user';

const def = computed(() => currentView());
const user = useUser();
</script>

<template>
  <div v-if="def?.page" class="app glim-shell page-app">
    <Head /><main id="page" class="page"><component :is="def.component" :arg="view.arg" /></main><BottomBar />
  </div>
  <div v-else-if="def" id="app" class="app glim-shell" :class="{ dual: prefs.dual, 'no-side': !prefs.side, 'with-fkeys': prefs.fkeys, 'side-open': shell.sideOpen }">
    <Head /><div id="main" class="main"><component :is="def.component" /></div><FKeys /><BottomBar />
  </div>
  <MenuHost /><WindowHost /><SignInReauthWindow :api="reauthClient" :text="reauthText()" /><SignInPage v-if="session.out" overlay :was="user.me.name" /><ToastHost />
</template>
