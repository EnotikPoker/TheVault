CREATE TABLE blackbox_puzzles (
    id BIGSERIAL PRIMARY KEY,
    puzzle_date DATE UNIQUE,
    rules_version INTEGER NOT NULL,
    layout BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE blackbox_sessions (
    id TEXT PRIMARY KEY,
    puzzle_id BIGINT NOT NULL REFERENCES blackbox_puzzles(id),
    state JSONB NOT NULL
);
