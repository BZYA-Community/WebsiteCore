import { request } from '@/utils/request';

export interface ColumnMedia {
  content: string;
  type: number;
  sort: number;
}
export interface ColumnSubmission {
  id: number;
  column_id: number;
  title: string;
  summary: string;
  body: string;
  cover: string;
  attachments: ColumnMedia[];
  audit_status: number;
  reason: string;
}
export interface ColumnItem {
  id: number;
  user_id: number;
  user?: Item.UserInfo;
  kind: 'article' | 'collection';
  title: string;
  summary: string;
  body: string;
  cover: string;
  attachments: ColumnMedia[];
  published_on: number;
  updated_on: number;
  submission_id: number;
  official: boolean;
  locked: boolean;
  blocked: boolean;
  curated: boolean;
  submission?: ColumnSubmission;
}
export interface ColumnDraft {
  title: string;
  summary: string;
  body: string;
  cover: string;
  attachments: ColumnMedia[];
}
export interface ColumnComment {
  id: number;
  column_id: number;
  parent_id: number;
  user_id: number;
  user: Item.UserInfo;
  at_user_id: number;
  at_user?: Item.UserInfo;
  content: string;
  contents: ColumnMedia[];
  ip_loc: string;
  audit_status: 0 | 1 | 2;
  reason: string;
  featured: boolean;
  likes: number;
  reaction: number;
  replies: ColumnComment[];
  created_on: number;
}
export interface ColumnReview {
  task: {
    id: number;
    kind: string;
    target_id: number;
    author_id: number;
    revision: number;
    assignee_id: number;
    deadline_on: number;
  };
  column: ColumnItem;
  submission?: ColumnSubmission;
  comment?: ColumnComment;
}
export interface ColumnListQuery {
  kind?: string;
  author_id?: number;
  keyword?: string;
  mine?: boolean;
  manage?: boolean;
  official?: boolean;
  page?: number;
  page_size?: number;
}
export const listColumns = (
  params: ColumnListQuery,
): Promise<{ items: ColumnItem[]; total: number }> =>
  request({ method: 'get', url: '/v1/columns', params });
export const getColumn = (
  id: number,
  editing = false,
): Promise<{ item: ColumnItem }> =>
  request({
    method: 'get',
    url: editing ? '/v1/column/submission' : '/v1/column',
    params: { id },
  });
export const submitColumn = (
  data: ColumnDraft & { id: number; kind: string; expected_submission: number },
): Promise<{ item: ColumnItem }> =>
  request({ method: 'post', url: '/v1/column/submit', data });
export const columnAction = (id: number, action: string): Promise<unknown> =>
  request({ method: 'post', url: '/v1/column/action', data: { id, action } });
export const columnEntries = (
  id: number,
  editing = false,
): Promise<{ items: ColumnItem[] }> =>
  request({
    method: 'get',
    url: '/v1/column/entries',
    params: { id, editing },
  });
export const setColumnEntries = (
  id: number,
  articles: number[],
): Promise<unknown> =>
  request({
    method: 'post',
    url: '/v1/column/entries',
    data: { id, articles },
  });
export const createColumnComment = (data: {
  column_id: number;
  parent_id?: number;
  at_user_id?: number;
  content?: string;
  contents?: Partial<Item.CommentItemProps>[];
  users?: string[];
}): Promise<{ audit_status: number }> =>
  request<unknown, { item: ColumnComment }>({
    method: 'post',
    url: '/v1/column/comment',
    data,
  }).then((res) => ({ audit_status: res.item.audit_status }));
export const columnCommentAction = (
  id: number,
  action: string,
): Promise<unknown> =>
  request({
    method: 'post',
    url: '/v1/column/comment/action',
    data: { id, action },
  });
export const listColumnReviews = (
  page: number,
): Promise<{ items: ColumnReview[]; total: number }> =>
  request({
    method: 'get',
    url: '/v1/admin/audit/columns',
    params: { page, page_size: 20 },
  });
export const decideColumnReview = (
  item: ColumnReview,
  action: string,
  reason: string,
): Promise<unknown> =>
  request({
    method: 'post',
    url: '/v1/admin/audit/column',
    data: {
      task_id: item.task.id,
      revision: item.task.revision,
      kind: item.task.kind,
      target_id: item.task.target_id,
      action,
      reason,
    },
  });

const reaction = (value: boolean): 0 | 1 => (value ? 1 : 0);
const replyProps = (c: ColumnComment): Item.ReplyProps => ({
  id: c.id,
  comment_id: c.parent_id,
  user_id: c.user_id,
  user: c.user,
  at_user_id: c.at_user_id,
  at_user: c.at_user || c.user,
  content: c.content,
  ip_loc: c.ip_loc,
  thumbs_up_count: c.likes,
  is_thumbs_up: reaction(c.reaction === 1),
  is_thumbs_down: reaction(c.reaction === -1),
  audit_status: c.audit_status,
  created_on: c.created_on,
});
export const columnCommentProps = (c: ColumnComment): Item.CommentProps => ({
  id: c.id,
  post_id: c.column_id,
  user_id: c.user_id,
  user: c.user,
  contents: (c.contents || []).map((item, index) => ({
    ...item,
    id: index + 1,
    comment_id: c.id,
    user_id: c.user_id,
    created_on: c.created_on,
  })),
  replies: (c.replies || []).map(replyProps),
  ip_loc: c.ip_loc,
  is_essence: reaction(c.featured),
  thumbs_up_count: c.likes,
  is_thumbs_up: reaction(c.reaction === 1),
  is_thumbs_down: reaction(c.reaction === -1),
  audit_status: c.audit_status,
  created_on: c.created_on,
});
export const getColumnComments = (
  id: number,
  style: string,
  page: number,
): Promise<{ items: Item.CommentProps[]; total: number }> =>
  request<unknown, { items: ColumnComment[]; total: number }>({
    method: 'get',
    url: '/v1/column/comments',
    params: { id, style, page, page_size: 20 },
  }).then((res) => ({
    items: res.items.map(columnCommentProps),
    total: res.total,
  }));
