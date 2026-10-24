import { request } from '@/utils/request';

export interface ProjectUser {
  id: number;
  username: string;
  nickname: string;
  avatar: string;
}

export interface ProjectMember {
  user: ProjectUser;
  role: 'owner' | 'member';
}

export interface ProjectItem {
  id: number;
  creator_id: number;
  creator: ProjectUser;
  post_id: number;
  title: string;
  intro: string;
  url: string;
  images: string[];
  tags: string[];
  members: ProjectMember[];
  follow_count: number;
  followed: boolean;
  created_on: number;
}

export interface ProjectPage {
  list: ProjectItem[];
  pager: Item.PagerProps;
}

export interface CreateProjectInput {
  title: string;
  intro: string;
  url: string;
  images: string[];
  tags: string[];
}

export const getProjects = (params: {
  keyword?: string;
  tag?: string;
  page: number;
  page_size: number;
}): Promise<ProjectPage> => request({ method: 'get', url: '/v1/projects', params });

export const getProject = (id: number): Promise<{ project: ProjectItem }> =>
  request({ method: 'get', url: '/v1/project', params: { id } });

export const createProject = (data: CreateProjectInput): Promise<ProjectItem> =>
  request({ method: 'post', url: '/v1/project', data });

export const followProject = (id: number): Promise<{ status: boolean; follow_count: number }> =>
  request({ method: 'post', url: '/v1/project/follow', data: { id } });

export const unfollowProject = (id: number): Promise<{ status: boolean; follow_count: number }> =>
  request({ method: 'post', url: '/v1/project/unfollow', data: { id } });
