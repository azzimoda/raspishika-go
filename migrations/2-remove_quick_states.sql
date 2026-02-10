UPDATE chats
SET state = 'default'
WHERE state IN ('quick_selecting_department', 'quick_selecting_group');
