-- Demo users for testing
-- Passwords are bcrypt hashed (cost=10)

-- Demo user (password: demo123)
INSERT INTO users (id, email, name, password_hash, created_at, updated_at) VALUES
    ('a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'demo@temper.dev', 'Demo User', '$2a$10$2RIYRvVPX.FbBvGIbETb2e/rVgyDCzpB4Qo7rttQLRrqXQRHAk//e', NOW() - INTERVAL '30 days', NOW()),

-- Alice - beginner learner (password: alice123)
    ('b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e', 'alice@temper.dev', 'Alice Johnson', '$2a$10$yaB.v5HbAqdcaqfT7Dk47elibDBZGhnChi7yZmsiTXX7tFxJjTwMO', NOW() - INTERVAL '14 days', NOW()),

-- Bob - intermediate learner (password: bob123)
    ('c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'bob@temper.dev', 'Bob Smith', '$2a$10$BJNv2PecSVfKnoedLNLI5eMAGXW1kHzMUZZWkWpTO5TaBdQGwN.Da', NOW() - INTERVAL '7 days', NOW());

-- Verify users were created
DO $$
BEGIN
    RAISE NOTICE 'Created % demo users', (SELECT COUNT(*) FROM users);
END $$;
