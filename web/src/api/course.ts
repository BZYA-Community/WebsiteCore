/**
 * 课程模块 API(手写封装, 与 @/api/post 同一风格; DELETE 动词 Proxy 客户端不支持故全部走这里)
 */

import { request } from '@/utils/request';
import axios from 'axios';
import { TOKEN_KEY } from '@/store/user';

export interface CourseUserBrief {
  id: number;
  username: string;
  nickname: string;
  avatar: string;
}

export interface CourseGroup {
  id: number;
  parent_id: number;
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
  teacher_intro: string;
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

export interface LessonAttachment {
  id: number;
  attachment_id: number;
  name: string;
  kind: 'attachment' | 'resource';
  file_size: number;
  mime_type: string;
}

export interface CourseLesson {
  id: number;
  course_id: number;
  title: string;
  intro: string;
  sort: number;
  attachments: LessonAttachment[];
}

export interface LessonInput {
  course_id: number;
  title: string;
  intro: string;
  sort: number;
  attachments: Pick<LessonAttachment, 'attachment_id' | 'name' | 'kind'>[];
}

export const getCourseLessons = (course_id: number): Promise<{ lessons: CourseLesson[] }> =>
  request({ method: 'get', url: '/v1/course/lessons', params: { course_id } });
export const createCourseLesson = (data: LessonInput): Promise<CourseLesson> =>
  request({ method: 'post', url: '/v1/admin/course/lesson', data });
export const updateCourseLesson = (data: LessonInput & { id: number }): Promise<CourseLesson> =>
  request({ method: 'post', url: '/v1/admin/course/lesson/update', data });
export const deleteCourseLesson = (id: number): Promise<unknown> =>
  request({ method: 'post', url: '/v1/admin/course/lesson/delete', data: { id } });
export const getCourseAttachment = (id: number): Promise<{ signed_url: string }> =>
  request({ method: 'get', url: '/v1/course/attachment', params: { id } });

interface CourseUploadTicket {
  attachment_id: number;
  mode: 'direct' | 'proxy';
  upload_url: string;
  method: 'POST' | 'PUT';
  fields?: Record<string, string>;
  expires_on: number;
}
export type UploadedCourseFile = Omit<LessonAttachment, 'id'>;
export async function uploadCourseFile(file: File, kind: LessonAttachment['kind'], signal: AbortSignal, progress: (value: number) => void): Promise<UploadedCourseFile> {
  const mime = file.type || 'application/octet-stream';
  const ticket = await request<unknown, CourseUploadTicket>({ method: 'post', url: '/v1/admin/course/upload/init', signal, data: { name: file.name, size: file.size, mime_type: mime, kind } });
  const base = new URL(import.meta.env.VITE_HOST || window.location.origin, window.location.origin);
  const url = new URL(ticket.upload_url, base);
  const headers: Record<string, string> = {};
  if (ticket.mode === 'proxy') {
    if (url.origin !== base.origin) throw new Error('Invalid proxy upload origin');
    headers.Authorization = `Bearer ${localStorage.getItem(TOKEN_KEY) || ''}`;
  }
  let data: File | FormData = file;
  if (ticket.method === 'POST') {
    const form = new FormData();
    for (const [name, value] of Object.entries(ticket.fields || {})) form.append(name, value);
    form.append('file', file);
    data = form;
  } else headers['Content-Type'] = mime;
  const uploaded = await axios.request({ url: url.href, method: ticket.method, data, headers, signal, onUploadProgress: (event) => { if (event.total) progress(Math.round(event.loaded * 100 / event.total)); } });
  if (ticket.mode === 'proxy' && uploaded.data?.code !== 0) throw new Error(uploaded.data?.msg || 'Upload failed');
  return request<unknown, UploadedCourseFile>({ method: 'post', url: '/v1/admin/course/upload/complete', signal, data: { attachment_id: ticket.attachment_id } });
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

/** Course questions and replies always enter review. */
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
  parent_id: number;
  sort: number;
}): Promise<CourseGroup> => {
  return request({ method: 'post', url: '/v1/admin/course/group', data });
};

export const updateCourseGroup = (data: {
  id: number;
  name: string;
  parent_id: number;
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
  teacher_intro: string;
  cover?: string;
}): Promise<CourseItem> => {
  return request({ method: 'post', url: '/v1/admin/course', data });
};

export const updateCourse = (data: {
  id: number;
  group_id: number;
  teacher_id: number;
  title: string;
  intro: string;
  teacher_intro: string;
  cover?: string;
}): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/update', data });
};

export const deleteCourse = (data: { id: number }): Promise<unknown> => {
  return request({ method: 'post', url: '/v1/admin/course/delete', data });
};
