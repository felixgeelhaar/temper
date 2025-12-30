-- Demo artifacts (workspaces) for testing

-- Demo user's workspaces
INSERT INTO artifacts (id, user_id, exercise_id, name, content, created_at, updated_at) VALUES
-- Completed Hello World exercise
('d4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f8a', 'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'go-v1/basics/hello-world', 'Hello World',
 '{"main.go": "package main\n\nimport \"fmt\"\n\nfunc Hello(name string) string {\n\tif name == \"\" {\n\t\tname = \"World\"\n\t}\n\treturn fmt.Sprintf(\"Hello, %s!\", name)\n}\n\nfunc main() {\n\tfmt.Println(Hello(\"\"))\n}\n", "main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) {\n\tgot := Hello(\"Alice\")\n\twant := \"Hello, Alice!\"\n\tif got != want {\n\t\tt.Errorf(\"Hello(Alice) = %q; want %q\", got, want)\n\t}\n}\n\nfunc TestHelloEmpty(t *testing.T) {\n\tgot := Hello(\"\")\n\twant := \"Hello, World!\"\n\tif got != want {\n\t\tt.Errorf(\"Hello(\"\"\") = %q; want %q\", got, want)\n\t}\n}\n"}',
 NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),

-- In-progress Variables exercise
('e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b', 'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'go-v1/basics/variables', 'Variables',
 '{"variables.go": "package main\n\n// TODO: Implement variable declarations\nvar x int\nvar y = 10\n\nfunc GetX() int {\n\treturn x\n}\n\nfunc GetY() int {\n\treturn y\n}\n"}',
 NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 day'),

-- Alice's workspaces (beginner)
-- Started Hello World
('f6a7b8c9-d0e1-4f2a-3b4c-5d6e7f8a9b0c', 'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e', 'go-v1/basics/hello-world', 'Hello World',
 '{"main.go": "package main\n\nfunc Hello(name string) string {\n\t// TODO: Implement greeting\n\treturn \"\"\n}\n"}',
 NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 days'),

-- Bob's workspaces (intermediate)
-- Completed multiple exercises
('a7b8c9d0-e1f2-4a3b-4c5d-6e7f8a9b0c1d', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'go-v1/basics/hello-world', 'Hello World',
 '{"main.go": "package main\n\nimport \"fmt\"\n\nfunc Hello(name string) string {\n\tif name == \"\" {\n\t\tname = \"World\"\n\t}\n\treturn fmt.Sprintf(\"Hello, %s!\", name)\n}\n"}',
 NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days'),

('b8c9d0e1-f2a3-4b4c-5d6e-7f8a9b0c1d2e', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'go-v1/basics/functions', 'Functions',
 '{"functions.go": "package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sum(nums ...int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\ttotal += n\n\t}\n\treturn total\n}\n"}',
 NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),

-- Bob working on table-driven tests
('c9d0e1f2-a3b4-4c5d-6e7f-8a9b0c1d2e3f', 'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'go-v1/testing/table-tests', 'Table Tests',
 '{"calculator.go": "package calculator\n\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a * b }\n", "calculator_test.go": "package calculator\n\nimport \"testing\"\n\n// TODO: Convert to table-driven tests\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Error(\"expected 5\")\n\t}\n}\n"}',
 NOW() - INTERVAL '1 day', NOW());

-- Verify artifacts were created
DO $$
BEGIN
    RAISE NOTICE 'Created % demo artifacts', (SELECT COUNT(*) FROM artifacts);
END $$;
