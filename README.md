# jira-servicedesk-enum

A Go tool for enumerating Atlassian Jira Service Desk users, checking user permissions, detecting leaked Confluence pages and triggering signups. Useful for security assessments and penetration testing. Brought to you by the [RasterSec](https://www.rastersec.com) team 🙌.

## Installation

```bash
go install github.com/RasterSec/jira-servicedesk-enum@latest
```

Or build from source:

```bash
go build
```

## Authentication

This tool uses the `customer.account.session.token` JWT cookie for authentication. The JWT is automatically parsed to extract our account ID for self-exclusion.

## Usage

### Signup

Trigger service desk signup:

```bash
./jira-servicedesk-enum signup \
  --url https://example.atlassian.net \
  --email user@example.com
```

### Check Permissions

Check what permissions we have:

```bash
./jira-servicedesk-enum permissions \
  --url https://example.atlassian.net \
  --cookie "secret..."
```

### Enumerate Users

#### Basic Usage

List users across all accessible service desks (default: max 50 per desk):

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..."
```

**Note**: Our own account is automatically excluded from results.

#### Export to CSV

Export results to a CSV file:

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --output users.csv
```

CSV format:

```csv
AccountID,DisplayName,Email,Avatar
qm:xxx:xxx:123,John Doe,john@example.com,https://...
```

#### Advanced Options

Target a specific service desk by ID:

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --desk 123
```

Fetch unlimited users (enables alphabet search):

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --max 0
```

Set a custom maximum per service desk:

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --max 100
```

Search with a custom query (skips automatic enumeration):

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --query "john"
```

Use a custom alphabet for search expansion:

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --alphabet "aeiou" \
  --max 0
```

Configure concurrent workers and timeouts:

```bash
./jira-servicedesk-enum users \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --workers 10 \
  --timeout 30
```

### Enumerate Confluence Pages

Sometimes internal documentation is exposed through the servicedesk.

#### Basic Usage

```bash
./jira-servicedesk-enum docs \
  --url https://example.atlassian.net \
  --cookie "secret..."
```

#### Advanced Options

Test with a single character first:

```bash
./jira-servicedesk-enum docs \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --alphabet "a"
```

Use two-tier alphabet system for efficient enumeration:

```bash
./jira-servicedesk-enum docs \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --alphabet "abcdefghijklmnopqrstuvwxyz0123456789" \
  --alphabet2 "abcdefghijklmnopqrstuvwxyz"
```

Configure concurrent workers and timeouts:

```bash
./jira-servicedesk-enum docs \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --workers 20 \
  --timeout 30
```

Export results to CSV:

```bash
./jira-servicedesk-enum docs \
  --url https://example.atlassian.net \
  --cookie "secret..." \
  --output docs.csv
```

## How It Works

### Alphabet Search Optimization

Jira's API returns a maximum of 50 users per query. The tool uses intelligent alphabet search to enumerate more users:

1. **Initial Query**: Starts with an empty query to fetch the first 50 users
2. **Smart Triggering**: Only activates alphabet search when:
   - The initial query returns exactly 50 users (indicating more exist), AND
   - `max` is set to 0 (unlimited) or > 50
3. **Two-Tier Expansion**: Uses a two-alphabet system for efficient enumeration:
   - **Layer 1** (default: `abcdefghijklmnopqrstuvwxyz0123456789`): Used for the first level of expansion
   - **Layer 2+** (default: `abcdefghijklmnopqrstuvwxyz`): Used for deeper recursion to reduce unnecessary API calls
4. **Concurrent Workers**: Processes multiple queries in parallel (default: 10 workers)

### Self-Exclusion

The tool automatically:

1. Parses the JWT cookie to extract your account ID from the `sub` field
2. Filters out your account from all results
3. Fails if JWT parsing fails (ensures accurate results)

### Graceful Shutdown

Press `Ctrl+C` at any time to gracefully stop enumeration and display results collected so far.

## Flags Reference

### Common Flags

- `--url`: Jira URL (required) - e.g., `https://example.atlassian.net`
- `--cookie`: Session cookie JWT (required for auth) - `customer.account.session.token`

### User Enumeration Flags

- `--max`: Maximum users per service desk (default: `50`, `0` = unlimited)
- `--desk`: Target specific service desk by ID (optional)
- `--query`: Custom search query - skips automatic enumeration (optional)
- `--alphabet`: Layer 1 alphabet for search expansion (default: `abcdefghijklmnopqrstuvwxyz0123456789`)
- `--alphabet2`: Layer 2+ alphabet for deeper search expansion (default: `abcdefghijklmnopqrstuvwxyz`)
- `--workers`: Number of concurrent workers (default: `10`)
- `--timeout`: HTTP request timeout in seconds (default: `10`)
- `--output`: Output CSV file path (optional)

### Document Enumeration Flags

- `--alphabet`: Layer 1 alphabet for search expansion (default: `abcdefghijklmnopqrstuvwxyz0123456789`)
- `--alphabet2`: Layer 2+ alphabet for deeper search expansion (default: `abcdefghijklmnopqrstuvwxyz`)
- `--workers`: Number of concurrent workers (default: `10`)
- `--timeout`: HTTP request timeout in seconds (default: `10`)
- `--output`: Output CSV file path (optional)

## License

Licensed under the Apache License, Version 2.0.
