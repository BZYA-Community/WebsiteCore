import { request } from '@/utils/request';

export interface IdentityGroup {
  id: number;
  key: string;
  name: string;
  description?: string;
  permissions: string[];
  builtin: boolean;
}

export interface PermissionItem {
  key: string;
  name: string;
}

export const getIdentity = (): Promise<{ permissions: string[] }> =>
  request({ method: 'get', url: '/v1/identity' });

export const getIdentityGroups = (): Promise<{ groups: IdentityGroup[] }> =>
  request({ method: 'get', url: '/v1/admin/identity/groups' });

export const getPermissionCatalog = (): Promise<{ permissions: PermissionItem[] }> =>
  request({ method: 'get', url: '/v1/admin/identity/permissions' });

export const saveIdentityGroup = (group: Pick<IdentityGroup, 'key' | 'name' | 'permissions' | 'description'> & { id?: number }) =>
  request({ method: 'post', url: '/v1/admin/identity/groups', data: group });

export const deleteIdentityGroup = (id: number) =>
  request({ method: 'delete', url: '/v1/admin/identity/groups', params: { id } });

export const setUserIdentity = (user_id: number, group_ids: number[]) =>
  request({ method: 'post', url: '/v1/admin/user/identity', data: { user_id, group_ids } });

export const setUsersIdentity = (user_ids: number[], group_ids: number[]) =>
  request({ method: 'post', url: '/v1/admin/user/identity', data: { user_ids, group_ids } });

export interface IdentityLog {
  id: number;
  actor_id: number;
  action: string;
  group_id: number;
  user_id: number;
  before: string;
  after: string;
  created_on: number;
}

export const getIdentityLogs = (page: number): Promise<{ list: IdentityLog[]; pager: { total_rows: number } }> =>
  request({ method: 'get', url: '/v1/admin/identity/logs', params: { page, page_size: 10 } });
