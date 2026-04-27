CREATE TYPE task_type AS ENUM (
    'todo', 'travel', 'meeting', 'reminder', 'focus', 'log'
);

CREATE TYPE task_status AS ENUM (
    'backlog', 'scheduled', 'in_progress', 'completed', 'cancelled'
);

CREATE TYPE task_priority AS ENUM (
    'highest', 'high', 'medium', 'low', 'lowest'
);

CREATE TYPE occurrence_type AS ENUM (
    'once', 'recurring', 'unbound'
);

CREATE TABLE tasks (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id       UUID          REFERENCES tasks(id) ON DELETE SET NULL,
    type            task_type     NOT NULL DEFAULT 'todo',
    title           TEXT          NOT NULL,
    details         TEXT,
    status          task_status,
    priority        task_priority,
    pinned          BOOLEAN       NOT NULL DEFAULT FALSE,
    archived        BOOLEAN       NOT NULL DEFAULT FALSE,
    occurrence_type occurrence_type NOT NULL DEFAULT 'once',
    start_at        TIMESTAMPTZ,
    end_at          TIMESTAMPTZ,
    recurrence_rule JSONB,
    location        TEXT,
    labels          TEXT[]        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_user_id    ON tasks(user_id);
CREATE INDEX idx_tasks_parent_id  ON tasks(parent_id);
CREATE INDEX idx_tasks_status     ON tasks(status);
CREATE INDEX idx_tasks_start_at   ON tasks(start_at);
CREATE INDEX idx_tasks_archived   ON tasks(archived);

CREATE TABLE task_links (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    label   TEXT,
    url     TEXT NOT NULL
);

CREATE INDEX idx_task_links_task_id ON task_links(task_id);

CREATE TABLE task_attachments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      UUID        NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    storage_key  TEXT        NOT NULL,
    filename     TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_attachments_task_id ON task_attachments(task_id);
