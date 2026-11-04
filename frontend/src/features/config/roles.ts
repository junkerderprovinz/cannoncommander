import type { Role, RuleKind, User } from '@/api/admin';
import { c } from './kit/kit';

export const ROLES: Role[] = ['admin', 'user', 'readonly'];
export const roleName = (r: Role) => c(`roles.${r}`);

export const RULES: [RuleKind, string][] = [['hide', 'IconEyeOff'], ['read', 'IconEye'], ['allow', 'IconCheck']];
export const ruleName = (r: RuleKind) => c(`rules.kind.${r}`);
export const ruleGlyph = (r: RuleKind) => (RULES.find((x) => x[0] === r) ?? RULES[0])[1];

// who is a user name, @group or * for everybody.
export const whoName = (who: string, users: User[]) => (who === '*' ? c('rules.everybody') : who.startsWith('@') ? c('rules.group', { name: who.slice(1) }) : users.find((u) => u.name === who)?.full || who);
