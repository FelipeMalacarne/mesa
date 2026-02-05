CREATE TABLE IF NOT EXISTS connections (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    driver TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL, -- AES-256 Encrypted
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
