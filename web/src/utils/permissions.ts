export interface PermissionSubject {
  permissions?: readonly string[];
  is_operator?: boolean;
}

export function hasPermission(subject: PermissionSubject, permission: string): boolean {
  return subject.is_operator === true || subject.permissions?.includes(permission) === true;
}

export function hasAnyPermission(subject: PermissionSubject, permissions: readonly string[]): boolean {
  return permissions.some((permission) => hasPermission(subject, permission));
}

// Guest/member membership is automatic; the assignment API accepts additional groups only.
export function isAssignableIdentityGroup(group: { key: string }): boolean {
  return group.key !== 'guest' && group.key !== 'member';
}
