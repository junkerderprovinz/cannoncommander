// What happens to a name that exists at the target, as the transfer window offers it and the conflict window answers it.
import type { Choice, JobConflict, Rule } from '@/api/ops';
import { t } from '@/i18n';

export const RULES = (): [Rule, string, string][] => [
  ['ask', t('transfer.rule.ask'), 'IconHelp'], ['skip', t('transfer.rule.skip'), 'IconForward'], ['replace', t('transfer.rule.replace'), 'IconSync'],
  ['newer', t('transfer.rule.newer'), 'IconUpdate'], ['both', t('transfer.rule.both'), 'IconCopy'],
];
// Two arrows in a circle swap the old for the new, an arrow ahead passes the name by, two sheets keep both.
export const CHOICES = () => [['replace', t('transfer.choice.replace'), 'IconSync'], ['skip', t('transfer.choice.skip'), 'IconForward'], ['both', t('transfer.choice.both'), 'IconCopy']] as const;

// Nur Neuere ersetzen answers each row by its dates; the other rules answer every row the same.
export const pick = (rule: Choice, c: Pick<JobConflict, 'newer'>): Choice => (rule === 'newer' ? (c.newer === 'source' ? 'replace' : 'skip') : rule);
