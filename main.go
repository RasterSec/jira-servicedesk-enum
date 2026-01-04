// Copyright 2025 İrem Kuyucu
// Copyright 2025 Laurynas Četyrkinas
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
)

var tenantSession *bool

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "signup":
		handleSignup()
	case "permissions":
		handlePermissions()
	case "users":
		handleUsers()
	case "docs":
		handleDocs()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: jira-servicedesk-enum <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  signup        Trigger servicedesk signup")
	fmt.Println("  permissions   Check user permissions")
	fmt.Println("  users         Enumerate users across service desks")
	fmt.Println("  docs          Enumerate exposed Confluence documentation")
	fmt.Println("\nRun 'jira-servicedesk-enum <command> -h' for command-specific help")
}

func handleSignup() {
	fs := flag.NewFlagSet("signup", flag.ExitOnError)
	url := fs.String("url", "", "Jira URL (e.g., https://example.atlassian.net)")
	email := fs.String("email", "", "Email address for signup")

	fs.Parse(os.Args[2:])

	if *url == "" || *email == "" {
		fmt.Fprintln(os.Stderr, "Error: --url and --email are required")
		fs.Usage()
		os.Exit(1)
	}

	if err := signup(*url, *email); err != nil {
		fmt.Fprintf(os.Stderr, "Error: signup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Signup successful, check email")
}

func handlePermissions() {
	fs := flag.NewFlagSet("permissions", flag.ExitOnError)
	url := fs.String("url", "", "Jira URL (e.g., https://example.atlassian.net)")
	cookie := fs.String("cookie", "", "Session cookie value (customer.account.session.token)")
	tenantSession = fs.Bool("tenantsession", false, "Set session cookie name to tenant.session.token")

	fs.Parse(os.Args[2:])

	if *url == "" || *cookie == "" {
		fmt.Fprintln(os.Stderr, "Error: --url and --cookie are required")
		fs.Usage()
		os.Exit(1)
	}

	if err := checkPermissions(*url, *cookie); err != nil {
		fmt.Fprintf(os.Stderr, "Error: permission check failed: %v\n", err)
		os.Exit(1)
	}
}

func handleUsers() {
	fs := flag.NewFlagSet("users", flag.ExitOnError)
	url := fs.String("url", "", "Jira URL (e.g., https://example.atlassian.net)")
	cookie := fs.String("cookie", "", "Session cookie value (customer.account.session.token)")
	maxUsers := fs.Int("max", 50, "Maximum users to fetch per service desk (0 = unlimited)")
	deskID := fs.String("desk", "", "Specific service desk ID to enumerate (optional)")
	query := fs.String("query", "", "Custom search query (optional, skips automatic enumeration)")
	alphabet1 := fs.String("alphabet", "abcdefghijklmnopqrstuvwxyz0123456789", "Alphabet for layer 1 search expansion")
	alphabet2 := fs.String("alphabet2", "abcdefghijklmnopqrstuvwxyz", "Alphabet for layer 2+ search expansion")
	workers := fs.Int("workers", 10, "Number of concurrent workers")
	timeout := fs.Int("timeout", 10, "HTTP request timeout in seconds")
	output := fs.String("output", "", "Output CSV file path (optional)")
	tenantSession = fs.Bool("tenantsession", false, "Set session cookie name to tenant.session.token")

	fs.Parse(os.Args[2:])

	if *url == "" || *cookie == "" {
		fmt.Fprintln(os.Stderr, "Error: --url and --cookie are required")
		fs.Usage()
		os.Exit(1)
	}

	selfAccountID, err := extractAccountIDFromJWT(*cookie)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not extract account ID from cookie: %v\n", err)
		os.Exit(1)
	}

	if err := enumerateUsers(*url, *cookie, *maxUsers, *deskID, *query, *alphabet1, *alphabet2, selfAccountID, *output, *workers, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: user enumeration failed: %v\n", err)
		os.Exit(1)
	}
}

func handleDocs() {
	fs := flag.NewFlagSet("docs", flag.ExitOnError)
	url := fs.String("url", "", "Jira URL (e.g., https://example.atlassian.net)")
	cookie := fs.String("cookie", "", "Session cookie value (customer.account.session.token)")
	alphabet1 := fs.String("alphabet", "abcdefghijklmnopqrstuvwxyz0123456789", "Alphabet for layer 1 search expansion")
	alphabet2 := fs.String("alphabet2", "abcdefghijklmnopqrstuvwxyz", "Alphabet for layer 2+ search expansion")
	workers := fs.Int("workers", 10, "Number of concurrent workers")
	timeout := fs.Int("timeout", 10, "HTTP request timeout in seconds")
	output := fs.String("output", "", "Output CSV file path (optional)")
	tenantSession = fs.Bool("tenantsession", false, "Set session cookie name to tenant.session.token")

	fs.Parse(os.Args[2:])

	if *url == "" || *cookie == "" {
		fmt.Fprintln(os.Stderr, "Error: --url and --cookie are required")
		fs.Usage()
		os.Exit(1)
	}

	if err := enumerateDocs(*url, *cookie, *alphabet1, *alphabet2, *output, *workers, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: document enumeration failed: %v\n", err)
		os.Exit(1)
	}
}
