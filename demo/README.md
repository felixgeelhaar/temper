# Temper Demo Environment

Pre-configured demo environment with seeded test accounts and data.

## Quick Start

```bash
# Start demo environment
docker-compose -f docker-compose.demo.yml up -d

# View logs
docker-compose -f docker-compose.demo.yml logs -f demo-api

# Stop demo environment
docker-compose -f docker-compose.demo.yml down

# Reset demo data (removes all data and re-seeds)
docker-compose -f docker-compose.demo.yml down -v
docker-compose -f docker-compose.demo.yml up -d
```

## Demo Accounts

| Email | Password | Description |
|-------|----------|-------------|
| `demo@temper.dev` | `demo123` | General demo account with mixed progress |
| `alice@temper.dev` | `alice123` | Beginner learner, just started |
| `bob@temper.dev` | `bob123` | Intermediate learner, good progress |

## Service Ports

Demo environment uses different ports to avoid conflicts with development:

| Service | Demo Port | Dev Port |
|---------|-----------|----------|
| API | 8081 | 8080 |
| PostgreSQL | 5433 | 5432 |
| RabbitMQ | 5673 | 5672 |
| RabbitMQ UI | 15673 | 15672 |
| Ollama | 11435 | 11434 |

## Demo Data

### Users
- 3 pre-created users with different skill levels
- bcrypt-hashed passwords (cost=10)

### Artifacts (Workspaces)
- Demo user: 2 workspaces (1 completed, 1 in-progress)
- Alice: 1 workspace (just started)
- Bob: 3 workspaces (multiple completed exercises)

### Learning Profiles
- Skill progression data per user
- Run statistics and hint usage
- Common error patterns tracked

### Runs (Execution History)
- 5 sample code execution records
- Mix of successful and failed runs
- Test output examples

## Customizing Demo Data

Edit files in `demo/seeds/`:
- `02_demo_users.sql` - User accounts
- `03_demo_artifacts.sql` - Workspaces with code
- `04_demo_profiles.sql` - Learning profiles
- `05_demo_runs.sql` - Execution history

To generate new password hashes:
```bash
go run -mod=mod -e 'package main; import ("fmt"; "golang.org/x/crypto/bcrypt"); func main() { h, _ := bcrypt.GenerateFromPassword([]byte("yourpassword"), 10); fmt.Println(string(h)) }'
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_PROVIDER` | ollama | LLM provider (claude, openai, ollama) |
| `LLM_API_KEY` | - | API key for cloud LLM providers |
| `LLM_MODEL` | llama3.2 | Model to use |

## Troubleshooting

### Database not seeding
```bash
# Check if seeds ran
docker-compose -f docker-compose.demo.yml exec demo-postgres psql -U temper -d temper -c "SELECT email FROM users;"

# Force re-seed by removing volume
docker-compose -f docker-compose.demo.yml down -v
docker-compose -f docker-compose.demo.yml up -d
```

### API not connecting to database
```bash
# Check database health
docker-compose -f docker-compose.demo.yml ps

# View API logs
docker-compose -f docker-compose.demo.yml logs demo-api
```
