-- +goose up
CREATE TABLE link_visits (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    referer TEXT NOT NULL DEFAULT '',
    status INTEGER NOT NULL
);

CREATE INDEX link_visits_link_id_idx ON link_visits(link_id);

-- +goose down
DROP TABLE link_visits;
