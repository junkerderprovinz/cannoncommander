import { plural, t } from '@/i18n';

export const files = (n: number) => (n ? plural(n, 'labels.files') : t('labels.noFile'));
