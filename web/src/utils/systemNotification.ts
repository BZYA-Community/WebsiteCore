export interface SystemNotification {
  senderName: string;
  profilePath?: string;
  postPath?: string;
  brief: string;
  content: string;
  timestamp: number;
}

/** Keep notification text separate from links; never interpret user text as HTML. */
export function toSystemNotification(
  message: Item.ChatHistoryItem,
): SystemNotification {
  return {
    senderName: message.sender_name || message.sender_username || '',
    profilePath:
      message.sender_id > 0 && message.sender_username
        ? `#/u?s=${encodeURIComponent(message.sender_username)}`
        : undefined,
    // Audit notifications are type 99 and carry the same post_id as comments.
    postPath:
      message.post_id && message.post_id > 0
        ? `#/post?id=${message.post_id}`
        : undefined,
    brief: message.brief || '',
    content: message.content,
    timestamp: message.timestamp,
  };
}
