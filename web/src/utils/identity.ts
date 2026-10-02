import i18n from '@/locales';

export interface UserIdentity {
    account_type?: 'member' | 'admin' | 'operator';
    roles?: string[];
    member_identity?: 'student' | 'teacher' | null;
    is_mentor?: boolean;
}

export const isAdmin = (user?: UserIdentity) =>
    !!user?.roles?.some((role) => role === 'operator' || role === 'admin');

export const isTeacher = (user?: UserIdentity) => user?.member_identity === 'teacher';
export const isMentor = (user?: UserIdentity) => !!user?.is_mentor && !isAdmin(user);
export const isOperator = (user?: UserIdentity) => !!user?.roles?.includes('operator');
export const canCreateCourse = (user?: UserIdentity) => isAdmin(user) || isTeacher(user);

export type IdentityTagType = 'error' | 'success' | 'info' | 'default';
export const identityTagType = (user?: UserIdentity): IdentityTagType =>
    isAdmin(user) ? 'error' : isTeacher(user) || isMentor(user) ? 'success' : 'info';

export const showIdentityBadge = (user?: UserIdentity) => isAdmin(user) || !!user?.member_identity;

// Auditor is only shown in management views, never in a public badge.
export const identityGroups = (user?: UserIdentity, includeAuditor = false): string[] => {
    if (user?.account_type === 'operator' || isOperator(user)) return ['operator'];
    if (user?.account_type === 'admin' || user?.roles?.includes('admin')) return ['admin'];
    const groups: string[] = [];
    if (isTeacher(user)) groups.push('teacher');
    if (isMentor(user)) groups.push('mentor');
    if (includeAuditor && user?.roles?.includes('auditor')) groups.push('auditor');
    if (!groups.length && user?.member_identity) groups.push('student');
    return groups;
};
export const identityLabels = (user?: UserIdentity, includeAuditor = false) =>
    identityGroups(user, includeAuditor).map((group) => i18n.global.t(`user.identity.${group}`));
export const identityLabel = (user?: UserIdentity, includeAuditor = false) => identityLabels(user, includeAuditor).join(' / ');
