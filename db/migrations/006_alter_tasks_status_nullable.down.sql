UPDATE tasks SET status = 'backlog' WHERE status IS NULL;
ALTER TABLE tasks ALTER COLUMN status SET NOT NULL;
