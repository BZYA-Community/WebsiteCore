/**
 * 课程模块 API(手写封装, 与 @/api/post 同一风格; DELETE 动词 Proxy 客户端不支持故全部走这里)
 */

import { request } from '@/utils/request';

export interface CourseUserBrief {
  id: number;
  username: string;
  nickname: string;
  avatar: string;
}

export interface CourseGroup {
  id: number;
  name: string;
  sort: number;
  course_count: number;
}

export interface CourseItem {
  id: number;
  group_id: number;
  group_name: string;
  teacher_id: number;
  teacher: CourseUserBrief;
  title: string;
  intro: string;
  cover: string;
  play_count: number;
  comment_count: number;
  created_on: number;
}

export interface CourseCommentContent {
  id: number;
  comment_id: number;
  user_id: number;
  content: string;
  type: number;
  sort: number;
}

export interface CourseReply {
  id: number;
  comment_id: number;
  user_id: number;
  user: CourseUserBrief;
  at_user_id: number;
  at_user: CourseUserBrief;
  content: string;
  ip_loc: string;
  audit_status: number;
  created_on: number;
  modified_on: number;
}

export interface CourseComment {
  id: number;
  course_id: number;
  user_id: number;
  user: CourseUserBrief;
  contents: CourseCommentContent[];
  replies: CourseReply[];
  ip_loc: string;
  reply_count: number;
  audit_status: number;
  created_on: number;
  modified_on: number;
}

export interface PageResp<T> {
  list: T[];
  pager: {
    page: number;
    page_size: number;
    total_rows: number;
  };
}

export interface UploadCredential {
  mode: 'direct' | 'proxy';
  host?: string;
  access_key_id?: string;
  policy?: string;
  signature?: string;
  key?: string;
  expire?: number;
}

/** 课程分组列表(含各组课程数) */
export const getCourseGroups = (): Promise<{ groups: CourseGroup[] }> => {
  return request({ method: 'get', url: '/v1/course/groups' });
};

/** 课程列表: group_id过滤分组, keyword搜索标题/简介 */
export const getCourseList = (params: {
  group_id?: number;
  keyword?: string;
  page: number;
  page_size: number;
}): Promise<PageResp<CourseItem>> => {
  return request({ method: 'get', url: '/v1/course/list', params });
};

/** 课程详情 */
export const getCourse = (params: {
  id: number;
}): Promise<{ course: CourseItem }> => {
  return request({ method: 'get', url: '/v1/course', params });
};

/** 课程评论列表 */
export const getCourseComments = (params: {
  id: number;
  page: number;
  page_size: number;
}): Promise<PageResp<CourseComment>> => {
  return request({ method: 'get', url: '/v1/course/comments', params });
};

/** 课程视频签名播放地址 */
export const getCourseVideo = (params: {
  id: number;
}): Promise<{ signed_url: string }> => {
  return request({ method: 'get', url: '/v1/course/video', params });
};

/** 播放计数(每次播放+1) */
export const playCourse = (data: {
  id: number;
}): Promise<{ play_count: number }> => {
  return request({ method: 'post', url: '/v1/course/play', data });
};

/** 发布课程评论(无角色用户进入审核) */
export const createCourseComment = (data: {
  course_id: number;
  contents: { content: string; type: number; sort: number }[];
  users: string[];
}): Promise<CourseComment> => {
  return request({ method: 'post', url: '/v1/course/comment', data });
};

/** 删除课程评论(本人或管理员) */
export const deleteCourseComment = (data: { id: number }): Promise<unknown> => {
  return request({ method: 'delete', url: '/v1/course/comment', data });
};

/** 回复课程评论 */
export const createCourseCommentReply = (data: {
  comment_id: number;
  at_user_id: number;
  content: string;
}): Promise<CourseReply> => {
  return request({ method: 'post', url: '/v1/course/comment/reply', data });
};

/** 删除课程回复(本人或管理员) */
export const deleteCourseCommentReply = (data: {
  id: number;
}): Promise<unknown> => {
  return request({ method: 'delete', url: '/v1/course/comment/reply', data });
};

// ===== 管理接口(管理员/运维) =====

export const createCourseGroup = (data: {
  name: string;
  sort: number;
}): Promise<CourseGroup> => {
  return request({ method: 'post', url: '/v1/admin/course/group', data });
};

export const updateCourseGroup = (data: {
  id: number;
  name: string;
  sort: number;
}): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/group/update', data });
};

export const deleteCourseGroup = (data: { id: number }): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/group/delete', data });
};

export const createCourse = (data: {
  group_id: number;
  teacher_id: number;
  title: string;
  intro: string;
  video: string;
  cover: string;
}): Promise<CourseItem> => {
  return request({ method: 'post', url: '/v1/admin/course', data });
};

export const updateCourse = (data: {
  id: number;
  group_id: number;
  teacher_id: number;
  title: string;
  intro: string;
  video?: string;
  cover?: string;
}): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/update', data });
};

export const deleteCourse = (data: { id: number }): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/delete', data });
};

/** 获取视频上传凭证(AliOSS返回直传policy, 其他返回proxy) */
export const getCourseUploadCredential = (params: {
  ext: string;
}): Promise<UploadCredential> => {
  return request({
    method: 'get',
    url: '/v1/admin/course/upload-credential',
    params,
  });
};
