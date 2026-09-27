-- name: CreateConversation :one
INSERT INTO chat_conversations (user_id)
VALUES (@user_id)
RETURNING *;

-- name: GetConversationByUUID :one
SELECT * FROM chat_conversations WHERE conversation_uuid = @conversation_uuid;

-- name: GetConversationByID :one
SELECT * FROM chat_conversations WHERE conversation_id = @conversation_id;

-- name: GetConversationByUserID :one
SELECT * FROM chat_conversations 
WHERE user_id = @user_id AND status != 'closed' 
ORDER BY created_at DESC 
LIMIT 1;

-- name: UpdateConversationStatus :one
UPDATE chat_conversations
SET status = @status, updated_at = now()
WHERE conversation_id = @conversation_id
RETURNING *;

-- name: UpdateLastMessageAt :exec
UPDATE chat_conversations
SET last_message_at = now(), updated_at = now()
WHERE conversation_id = @conversation_id;

-- name: ListConversations :many
SELECT 
    c.conversation_id,
    c.conversation_uuid,
    c.user_id,
    c.admin_id,
    c.status,
    c.last_message_at,
    c.created_at,
    c.updated_at,
    u.user_email,
    u.user_fullname,
    u.user_avatar,
    COALESCE((
        SELECT COUNT(*) 
        FROM chat_messages m 
        WHERE m.conversation_id = c.conversation_id 
          AND m.is_read = FALSE 
          AND m.sender_role = 'user'
    ), 0)::bigint AS unread_count,
    COALESCE((
        SELECT m.content 
        FROM chat_messages m 
        WHERE m.conversation_id = c.conversation_id 
        ORDER BY m.message_id DESC 
        LIMIT 1
    ), '')::text AS last_message_content
FROM chat_conversations c
JOIN users u ON u.user_id = c.user_id
WHERE (@status::text = '' OR c.status = @status)
  AND (
      @search::text = '' 
      OR u.user_fullname ILIKE '%' || @search || '%' 
      OR u.user_email ILIKE '%' || @search || '%'
  )
ORDER BY c.last_message_at DESC NULLS LAST
LIMIT @limit_count OFFSET @offset_count;

-- name: CountConversations :one
SELECT COUNT(*) 
FROM chat_conversations c
JOIN users u ON u.user_id = c.user_id
WHERE (@status::text = '' OR c.status = @status)
  AND (
      @search::text = '' 
      OR u.user_fullname ILIKE '%' || @search || '%' 
      OR u.user_email ILIKE '%' || @search || '%'
  );

-- name: CreateMessage :one
INSERT INTO chat_messages (conversation_id, sender_id, sender_role, content, content_type)
VALUES (@conversation_id, @sender_id, @sender_role, @content, @content_type)
RETURNING *;

-- name: ListMessages :many
SELECT * FROM chat_messages
WHERE conversation_id = @conversation_id
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListMessagesCursor :many
SELECT * FROM chat_messages
WHERE conversation_id = @conversation_id
  AND (@before_id::int = 0 OR message_id < @before_id)
ORDER BY message_id DESC
LIMIT @limit_count;

-- name: CountMessagesByConversationID :one
SELECT COUNT(*) FROM chat_messages
WHERE conversation_id = @conversation_id;

-- name: MarkMessagesRead :exec
UPDATE chat_messages
SET is_read = TRUE, read_at = now()
WHERE conversation_id = @conversation_id
  AND sender_id != @reader_id
  AND is_read = FALSE;

-- name: CountUnreadMessages :one
SELECT COUNT(*) FROM chat_messages
WHERE conversation_id = @conversation_id
  AND is_read = FALSE
  AND sender_id != @reader_id;

-- name: GetTotalUnreadMessagesForUser :one
SELECT COUNT(*) FROM chat_messages m
JOIN chat_conversations c ON c.conversation_id = m.conversation_id
WHERE c.user_id = @user_id 
  AND m.sender_role = 'admin' 
  AND m.is_read = FALSE;

-- name: GetTotalUnreadMessagesForAdmin :one
SELECT COUNT(*) FROM chat_messages
WHERE sender_role = 'user' 
  AND is_read = FALSE;
