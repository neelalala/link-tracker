CREATE TABLE IF NOT EXISTS chats (
    id BIGINT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS links (
    id BIGSERIAL PRIMARY KEY,
    url TEXT UNIQUE NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_link_url ON links(url);

CREATE TABLE IF NOT EXISTS subscriptions (
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    link_id BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, link_id)
);

CREATE INDEX IF NOT EXISTS idx_subscription_chat_id ON subscriptions(chat_id);
CREATE INDEX IF NOT EXISTS idx_subscription_link_id ON subscriptions(link_id);

CREATE TABLE IF NOT EXISTS subscription_tags (
    chat_id BIGINT NOT NULL,
    link_id BIGINT NOT NULL,
    tag VARCHAR(255) NOT NULL,

    FOREIGN KEY (chat_id, link_id) REFERENCES subscriptions(chat_id, link_id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, link_id, tag)
);

CREATE TABLE IF NOT EXISTS sessions (
    chat_id BIGINT PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    url TEXT NOT NULL,

    CONSTRAINT valid_session_state CHECK (
        state IN ('idle', 'waiting_for_url_track', 'waiting_for_tags', 'waiting_for_url_untrack')
    )
)