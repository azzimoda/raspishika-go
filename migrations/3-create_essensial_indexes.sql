-- chats
CREATE INDEX IF NOT EXISTS idx_chats_tg_chat_id ON chats(tg_chat_id);
CREATE INDEX IF NOT EXISTS idx_chats_group ON chats("group");
CREATE INDEX IF NOT EXISTS idx_chats_daily_sending_time ON chats(daily_sending_time);

-- groups
CREATE INDEX IF NOT EXISTS idx_groups_group_name ON groups(group_name);
CREATE INDEX IF NOT EXISTS idx_groups_updated_at ON groups(updated_at);

-- teachers
CREATE INDEX IF NOT EXISTS idx_teachers_name ON teachers(name);
CREATE INDEX IF NOT EXISTS idx_teachers_updated_at ON teachers(updated_at);

-- recent_teachers
CREATE INDEX IF NOT EXISTS idx_recent_teachers_chat_id ON recent_teachers(chat_id);
CREATE INDEX IF NOT EXISTS idx_recent_teachers_created_at ON recent_teachers(created_at);

-- update_logs
CREATE INDEX IF NOT EXISTS idx_update_logs_chat_id ON update_logs(chat_id);
CREATE INDEX IF NOT EXISTS idx_update_logs_created_at ON update_logs(created_at);
