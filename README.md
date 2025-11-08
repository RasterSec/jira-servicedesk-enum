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

## How It Works

### Alphabet Search Optimization

Jira's API returns a maximum of 50 users per query. The tool uses intelligent alphabet search to enumerate more users:

1. **Initial Query**: Starts with an empty query to fetch the first 50 users
2. **Smart Triggering**: Only activates alphabet search when:
   - The initial query returns exactly 50 users (indicating more exist), AND
   - `max` is set to 0 (unlimited) or > 50
3. **Recursive Expansion**: Appends alphabet characters (`abcdefghijklmnopqrstuvwxyz0123456789`) to queries until all users are found

### Self-Exclusion

The tool automatically:
1. Parses the JWT cookie to extract your account ID from the `sub` field
2. Filters out your account from all results
3. Fails if JWT parsing fails (ensures accurate results)

## Flags Reference

### Common Flags
- `--url`: Jira URL (required) - e.g., `https://example.atlassian.net`
- `--cookie`: Session cookie JWT (required for auth) - `customer.account.session.token`

### User Enumeration Flags
- `--max`: Maximum users per service desk (default: `50`, `0` = unlimited)
- `--desk`: Target specific service desk by ID (optional)
- `--query`: Custom search query - skips automatic enumeration (optional)
- `--alphabet`: Custom alphabet for search expansion (default: `abcdefghijklmnopqrstuvwxyz0123456789`)
- `--output`: Output CSV file path (optional)

## License

Licensed under the Apache License, Version 2.0.
