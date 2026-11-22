<script setup lang="ts">
// The own account's ways in, as GlimStone's sign-in tile draws them on the kit's sign-in. The Users tab manages the
// accounts, so the tile leaves their card out.
import { SignInTile } from '@glimstone/vue';
import type { SignInTileCard, ToastSeverity } from '@glimstone/core';
import { IDENTITY_PROVIDERS } from '@glimstone/core/identityProviders';
import { computed } from 'vue';
import { fmtTime } from '@/core/format';
import { toast } from '@/core/toast';
import { locale } from '@/i18n';
import { signIn } from '@/signin/kit';
import { tileText } from '@/signin/words';
import { takeArrived } from '../arrived';

const CARDS: SignInTileCard[] = ['password', 'askPassword', 'twoFactor', 'passkeys', 'sessions', 'tokens', 'services'];
const KEYS = {
  change: 'IconPencil', setPassword: 'IconKey', removePassword: 'IconKeyRevoke', savePassword: 'IconSave', setUp: 'IconShieldOn',
  newCodes: 'IconRefresh', turnOff: 'IconShieldOff', copy: 'IconCopy', confirm: 'IconConfirm', add: 'IconAdd', rename: 'IconPencil',
  remove: 'IconTrash', end: 'IconSignOut', endOthers: 'IconSignOut', createToken: 'IconAdd', revoke: 'IconKeyRevoke',
  addService: 'IconAdd', test: 'IconSendTest', edit: 'IconPencil', turnOn: 'IconSignIn', save: 'IconSave', block: 'IconShieldOff', unblock: 'IconShieldOn', passkey: 'IconKey',
  token: 'IconKey', account: 'IconAccounts', laptop: 'IconLaptop', phone: 'IconPhone', service: 'IconSignIn', forward: 'IconForward',
};

const KIND = { success: 'ok', warn: 'warn', fail: 'fault' } as const;

const text = computed(tileText);
const notify = (message: string, severity: ToastSeverity) => toast(message, KIND[severity]);
const arrived = takeArrived();
const time = (iso: string) => fmtTime(Date.parse(iso));
</script>

<template>
  <SignInTile
    :api="signIn"
    :text="text"
    :lang="locale()"
    :providers="IDENTITY_PROVIDERS"
    :cards="CARDS"
    :scopes="['read', 'all']"
    :notify="notify"
    :arrived="arrived"
    :time="time"
    :label-keys="KEYS"
  />
</template>
