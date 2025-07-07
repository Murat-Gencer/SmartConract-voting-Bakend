CREATE USER username WITH PASSWORD  'PASSWORD';
CREATE DATABASE voting_db OWNER username;

CREATE TABLE polls (
    id SERIAL PRIMARY KEY,
    question TEXT NOT NULL,
    options TEXT[] NOT NULL,
    creator VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    signature VARCHAR(255) NOT NULL UNIQUE,
    image_path VARCHAR(500),
    poll_address VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE votes (
    id SERIAL PRIMARY KEY,
    poll_id INTEGER NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    voter_address VARCHAR(255) NOT NULL,
    option_index INTEGER NOT NULL,
    vote_signature VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_voter_per_poll UNIQUE(poll_id, voter_address)
);