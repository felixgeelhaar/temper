-- Demo learning profiles

INSERT INTO learning_profiles (id, user_id, topic_skills, total_exercises, total_runs, hint_requests, avg_time_to_green_ms, common_errors, updated_at) VALUES
-- Demo user - some progress
('d0e1f2a3-b4c5-4d6e-7f8a-9b0c1d2e3f4a', 'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d',
 '{"basics/hello-world": 0.8, "basics/variables": 0.4, "basics/functions": 0.2}',
 2, 15, 3, 180000, ARRAY['missing return statement', 'undefined variable'], NOW()),

-- Alice - beginner, just started
('e1f2a3b4-c5d6-4e7f-8a9b-0c1d2e3f4a5b', 'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
 '{"basics/hello-world": 0.3}',
 1, 5, 2, 300000, ARRAY['syntax error', 'missing package'], NOW()),

-- Bob - intermediate, good progress
('f2a3b4c5-d6e7-4f8a-9b0c-1d2e3f4a5b6c', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f',
 '{"basics/hello-world": 1.0, "basics/variables": 0.9, "basics/functions": 0.85, "testing/table-tests": 0.5}',
 4, 42, 5, 120000, ARRAY['test assertion', 'nil pointer'], NOW());

-- Verify profiles were created
DO $$
BEGIN
    RAISE NOTICE 'Created % demo learning profiles', (SELECT COUNT(*) FROM learning_profiles);
END $$;
