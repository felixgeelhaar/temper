-- Demo runs (code execution history)

INSERT INTO runs (id, artifact_id, user_id, exercise_id, status, recipe, output, started_at, finished_at, created_at) VALUES
-- Demo user's successful run
('10a1b2c3-d4e5-4f6a-7b8c-9d0e1f2a3b4c', 'd4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f8a', 'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'go-v1/basics/hello-world',
 'completed',
 '{"format": true, "build": true, "test": true, "timeout": 30}',
 '{"format": {"ok": true}, "build": {"ok": true}, "test": {"ok": true, "output": "=== RUN   TestHello\n--- PASS: TestHello (0.00s)\n=== RUN   TestHelloEmpty\n--- PASS: TestHelloEmpty (0.00s)\nPASS\nok  \ttemper\t0.002s\n", "duration_ms": 1523}}',
 NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days' + INTERVAL '2 seconds', NOW() - INTERVAL '5 days'),

-- Demo user's failed run (during learning)
('20b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b', 'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'go-v1/basics/variables',
 'completed',
 '{"format": true, "build": true, "test": true, "timeout": 30}',
 '{"format": {"ok": true}, "build": {"ok": false, "output": "variables.go:5:5: undefined: z\n"}}',
 NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '1 second', NOW() - INTERVAL '2 days'),

-- Alice's run with test failure
('30c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e', 'f6a7b8c9-d0e1-4f2a-3b4c-5d6e7f8a9b0c', 'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e', 'go-v1/basics/hello-world',
 'completed',
 '{"format": true, "build": true, "test": true, "timeout": 30}',
 '{"format": {"ok": true}, "build": {"ok": true}, "test": {"ok": false, "output": "=== RUN   TestHello\n--- FAIL: TestHello (0.00s)\n    main_test.go:8: Hello(Alice) = \"\"; want \"Hello, Alice!\"\nFAIL\nexit status 1\n", "duration_ms": 1102}}',
 NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '1 second', NOW() - INTERVAL '2 days'),

-- Bob's multiple successful runs
('40d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'a7b8c9d0-e1f2-4a3b-4c5d-6e7f8a9b0c1d', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'go-v1/basics/hello-world',
 'completed',
 '{"format": true, "build": true, "test": true, "timeout": 30}',
 '{"format": {"ok": true}, "build": {"ok": true}, "test": {"ok": true, "output": "PASS\nok  \ttemper\t0.001s\n", "duration_ms": 987}}',
 NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days' + INTERVAL '1 second', NOW() - INTERVAL '6 days'),

('50e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f8a', 'b8c9d0e1-f2a3-4b4c-5d6e-7f8a9b0c1d2e', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'go-v1/basics/functions',
 'completed',
 '{"format": true, "build": true, "test": true, "timeout": 30}',
 '{"format": {"ok": true}, "build": {"ok": true}, "test": {"ok": true, "output": "PASS\nok  \ttemper\t0.002s\n", "duration_ms": 1245}}',
 NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days' + INTERVAL '2 seconds', NOW() - INTERVAL '4 days');

-- Verify runs were created
DO $$
BEGIN
    RAISE NOTICE 'Created % demo runs', (SELECT COUNT(*) FROM runs);
END $$;
