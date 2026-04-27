-- Seed data for local development
-- Users password: password123
-- bcrypt hash generated at cost 10

-- ── Users ─────────────────────────────────────────────────────────────────────

INSERT INTO users (id, email, password_hash, created_at) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'alice@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', NOW()),
    ('a0000000-0000-0000-0000-000000000002', 'bob@example.com',   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', NOW())
ON CONFLICT (email) DO NOTHING;

-- ── Tasks (Alice) ─────────────────────────────────────────────────────────────

INSERT INTO tasks (id, user_id, type, title, details, status, priority, occurrence_type, start_at, end_at, labels, created_at, updated_at) VALUES

    -- todos
    ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001',
     'todo', 'Buy groceries', 'Milk, eggs, bread, coffee', 'backlog', 'medium', 'once',
     NULL, NULL, '{"shopping","errands"}', NOW(), NOW()),

    ('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001',
     'todo', 'Submit tax return', 'Use last year documents', 'in_progress', 'highest', 'once',
     NOW(), NOW() + INTERVAL '3 days', '{"finance"}', NOW(), NOW()),

    ('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000001',
     'todo', 'Read Clean Architecture book', NULL, 'completed', 'low', 'once',
     NULL, NULL, '{"learning"}', NOW() - INTERVAL '7 days', NOW()),

    -- meeting
    ('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000001',
     'meeting', 'Team standup', 'Daily sync with the team', 'scheduled', 'medium', 'recurring',
     NOW() + INTERVAL '1 day', NOW() + INTERVAL '1 day' + INTERVAL '30 minutes',
     '{"work","daily"}', NOW(), NOW()),

    -- focus
    ('b0000000-0000-0000-0000-000000000005', 'a0000000-0000-0000-0000-000000000001',
     'focus', 'Deep work: API refactor', 'No interruptions', 'scheduled', 'high', 'once',
     NOW() + INTERVAL '2 hours', NOW() + INTERVAL '4 hours',
     '{"work","dev"}', NOW(), NOW()),

    -- reminder
    ('b0000000-0000-0000-0000-000000000006', 'a0000000-0000-0000-0000-000000000001',
     'reminder', 'Call dentist', NULL, 'backlog', 'low', 'once',
     NULL, NULL, '{}', NOW(), NOW()),

    -- travel
    ('b0000000-0000-0000-0000-000000000007', 'a0000000-0000-0000-0000-000000000001',
     'travel', 'Flight to Singapore', 'Bring passport and boarding pass', 'scheduled', 'highest', 'once',
     NOW() + INTERVAL '14 days', NOW() + INTERVAL '14 days' + INTERVAL '8 hours',
     '{"travel","trip"}', NOW(), NOW()),

    -- log (no status/priority — unbound)
    ('b0000000-0000-0000-0000-000000000008', 'a0000000-0000-0000-0000-000000000001',
     'log', 'Finished onboarding docs', 'Reviewed all internal wikis', NULL, NULL, 'unbound',
     NOW() - INTERVAL '1 day', NULL,
     '{work}', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- sub-task of Buy groceries
    ('b0000000-0000-0000-0000-000000000009', 'a0000000-0000-0000-0000-000000000001',
     'todo', 'Pick up dry cleaning', NULL, 'backlog', 'lowest', 'once',
     NULL, NULL, '{"errands"}', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- attach sub-task to parent
UPDATE tasks SET parent_id = 'b0000000-0000-0000-0000-000000000001'
WHERE id = 'b0000000-0000-0000-0000-000000000009';

-- recurring rule for team standup
UPDATE tasks
SET recurrence_rule = '{"frequency":"daily","interval":1,"days_of_week":["mon","tue","wed","thu","fri"]}'
WHERE id = 'b0000000-0000-0000-0000-000000000004';

-- ── Tasks (Bob) ───────────────────────────────────────────────────────────────

INSERT INTO tasks (id, user_id, type, title, status, priority, occurrence_type, created_at, updated_at) VALUES
    ('b0000000-0000-0000-0000-000000000010', 'a0000000-0000-0000-0000-000000000002',
     'todo', 'Set up dev environment', 'completed', 'high', 'once', NOW() - INTERVAL '2 days', NOW()),

    ('b0000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000002',
     'todo', 'Review pull requests', 'in_progress', 'medium', 'once', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ── Analytics events ──────────────────────────────────────────────────────────

INSERT INTO analytics_events (user_id, event_type, payload, occurred_at) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'task.created',   '{"task_id":"b0000000-0000-0000-0000-000000000001"}', NOW() - INTERVAL '5 days'),
    ('a0000000-0000-0000-0000-000000000001', 'task.created',   '{"task_id":"b0000000-0000-0000-0000-000000000002"}', NOW() - INTERVAL '4 days'),
    ('a0000000-0000-0000-0000-000000000001', 'task.created',   '{"task_id":"b0000000-0000-0000-0000-000000000003"}', NOW() - INTERVAL '7 days'),
    ('a0000000-0000-0000-0000-000000000001', 'task.completed', '{"task_id":"b0000000-0000-0000-0000-000000000003"}', NOW() - INTERVAL '3 days'),
    ('a0000000-0000-0000-0000-000000000002', 'task.created',   '{"task_id":"b0000000-0000-0000-0000-000000000010"}', NOW() - INTERVAL '2 days'),
    ('a0000000-0000-0000-0000-000000000002', 'task.completed', '{"task_id":"b0000000-0000-0000-0000-000000000010"}', NOW() - INTERVAL '1 day');
