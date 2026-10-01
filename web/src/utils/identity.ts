import i18n from '@/locales';

export interface UserIdentity {
    roles?: string[];
    member_identity?: 'student' | 'teacher' | null;
    is_mentor?: boolean;
}

export const isAdmin = (user?: UserIdentity) =>
    !!user?.roles?.some((role) => role === 'operator' || role === 'admin');

export const isTeacher = (user?: UserIdentity) => user?.member_identity === 'teacher';

export type IdentityTagType = 'error' | 'success' | 'info' | 'default';
export const identityTagType = (user?: UserIdentity): IdentityTagType =>
    isAdmin(user) ? 'error' : isTeacher(user) ? 'success' : 'info';

export const showIdentityBadge = (user?: UserIdentity) => isAdmin(user) || !!user?.member_identity;

// Public labels never depend on Auditor, including when rendering self data.
export const identityLabel = (user?: UserIdentity): string => {
    if (user?.roles?.includes('operator')) return i18n.global.t('user.identity.operator');
    if (user?.roles?.includes('admin')) return i18n.global.t('user.identity.admin');
    if (user?.member_identity === 'teacher') return i18n.global.t('user.identity.teacher');
    if (user?.member_identity === 'student') return i18n.global.t('user.identity.student');
    return '';
};
