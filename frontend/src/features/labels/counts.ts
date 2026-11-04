import { t } from '@/i18n';

export const files = (n: number) => (n ? (n === 1 ? t('labels.oneFile') : t('labels.files', { n })) : t('labels.noFile'));
