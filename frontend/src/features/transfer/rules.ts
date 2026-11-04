// What happens to a name that exists at the target, as the transfer window offers it and the conflict window answers it.
import type { Choice, JobConflict, Rule } from '@/api/ops';
import { t } from '@/i18n';

export const RULES = (): [Rule, string, string][] => [
  ['ask', t('transfer.rule.ask'), 'IconHelp'], ['skip', t('transfer.rule.skip'), 'IconKeepRight'], ['replace', t('transfer.rule.replace'), 'IconKeepLeft'],
  ['newer', t('transfer.rule.newer'), 'IconUpdate'], ['both', t('transfer.rule.both'), 'IconKeepBoth'],
];
export const CHOICES = () => [['replace', t('transfer.choice.replace'), 'IconKeepLeft'], ['skip', t('transfer.choice.skip'), 'IconKeepRight'], ['both', t('transfer.choice.both'), 'IconKeepBoth']] as const;

// Nur Neuere ersetzen answers each row by its dates; the other rules answer every row the same.
export const pick = (rule: Choice, c: Pick<JobConflict, 'newer'>): Choice => (rule === 'newer' ? (c.newer === 'source' ? 'replace' : 'skip') : rule);
