export interface SystemNotification {
  senderName: string;
  profilePath?: string;
  postPath?: string;
  columnPath?: string;
  brief: string;
  content: string;
  timestamp: number;
}

/** Keep notification text separate from links; never interpret user text as HTML. */
export function toSystemNotification(
  message: Item.ChatHistoryItem,
): SystemNotification {
  const comment = message.comment_id ? `&comment_id=${message.comment_id}` : '';
  const columnPath =
    message.column_id && message.column_id > 0
      ? `#/columns?view=${message.column_returned ? 'returned' : message.type === 99 && !message.sender_id && !message.comment_id ? 'edit' : 'read'}&id=${message.column_id}${comment}`
      : undefined;
  return {
    senderName: message.sender_name || message.sender_username || '',
    profilePath:
      message.sender_id > 0 && message.sender_username
        ? `#/u?s=${encodeURIComponent(message.sender_username)}`
        : undefined,
    // Audit/comment notifications carry post_id; column notices use columnPath instead.
    postPath:
      !columnPath && message.post_id && message.post_id > 0
        ? `#/post?id=${message.post_id}`
        : undefined,
    columnPath,
    brief: message.brief || '',
    content: message.content,
    timestamp: message.timestamp,
  };
}
