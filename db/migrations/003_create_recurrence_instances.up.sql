CREATE TYPE instance_status AS ENUM (
    'pending', 'completed', 'skipped'
);

CREATE TABLE recurrence_instances (
    id           UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      UUID            NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    scheduled_at TIMESTAMPTZ     NOT NULL,
    status       instance_status NOT NULL DEFAULT 'pending',
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_recurrence_instances_task_id      ON recurrence_instances(task_id);
CREATE INDEX idx_recurrence_instances_scheduled_at ON recurrence_instances(scheduled_at);
