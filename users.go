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
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type ServiceDeskResponse struct {
	Values []ServiceDesk `json:"values"`
}

type ServiceDesk struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	ProjectKey  string `json:"projectKey"`
}

type User struct {
	ID           string `json:"id"`
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	Avatar       string `json:"avatar"`
}

const defaultAvatar = "/default-avatar.png"

type userSearchTask struct {
	deskID string
	query  string
	depth  int
}

type userSearchResult struct {
	deskID string
	query  string
	depth  int
	users  []User
	err    error
}

func enumerateUsers(baseURL, cookie string, maxUsers int, deskID string, customQuery string, alphabet1, alphabet2 string, selfAccountID string, outputPath string, workers, timeout int) error {
	client := newClient(baseURL, cookie, time.Duration(timeout)*time.Second)

	var desks []ServiceDesk
	targetingSingleDesk := deskID != ""

	if targetingSingleDesk {
		desks = []ServiceDesk{{ID: deskID}}
	} else {
		resp, err := client.get("/rest/servicedeskapi/servicedesk")
		if err != nil {
			return fmt.Errorf("get service desks: %w", err)
		}

		var desksResp ServiceDeskResponse
		if err := unmarshalJSON(resp, &desksResp); err != nil {
			return fmt.Errorf("parse service desks: %w", err)
		}

		desks = desksResp.Values
		fmt.Printf("\nFound %d service desk(s)\n", len(desks))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interruptedChan := setupSignalHandler(cancel)

	userMap := make(map[string]User)

	for _, desk := range desks {
		select {
		case <-interruptedChan:
			break
		default:
		}

		if desk.ProjectName != "" {
			fmt.Println("\nService Desk: " + desk.ProjectName + " (" + desk.ProjectKey + ") [ID: " + desk.ID + "]")
		} else {
			fmt.Println("\nService Desk: [ID: " + desk.ID + "]")
		}

		totalFetched := 0
		capped := false
		seenAccountIDs := make(map[string]bool)
		searchCount := 0

		taskQueue := make(chan userSearchTask, 5000)
		results := make(chan userSearchResult, workers*2)

		// Worker pool
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case task, ok := <-taskQueue:
						if !ok {
							return
						}

						var url string
						if task.query == "" {
							url = fmt.Sprintf("/rest/servicedesk/1/customer/portal/%s/user-search/proforma", task.deskID)
						} else {
							url = fmt.Sprintf("/rest/servicedesk/1/customer/portal/%s/user-search/proforma?query=%s", task.deskID, task.query)
						}

						resp, err := client.get(url)
						if err != nil {
							select {
							case results <- userSearchResult{deskID: task.deskID, query: task.query, depth: task.depth, err: err}:
							case <-ctx.Done():
								return
							}
							continue
						}

						var users []User
						if err := unmarshalJSON(resp, &users); err != nil {
							select {
							case results <- userSearchResult{deskID: task.deskID, query: task.query, depth: task.depth, err: err}:
							case <-ctx.Done():
								return
							}
							continue
						}

						select {
						case results <- userSearchResult{deskID: task.deskID, query: task.query, depth: task.depth, users: users}:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
		}

		// Results processor - single goroutine, no mutexes needed for its own state
		var processorWg sync.WaitGroup
		processorWg.Add(1)
		go func() {
			defer processorWg.Done()
			pendingTasks := 1 // Start with 1 (initial query)

			for result := range results {
				pendingTasks-- // This task completed
				searchCount++

				if result.err != nil {
					fmt.Fprintf(os.Stderr, "Warning: search for '%s' failed: %v\n", result.query, result.err)
					if pendingTasks == 0 {
						cancel()
						return
					}
					continue
				}

				newUsersThisBatch := 0
				for _, user := range result.users {
					if seenAccountIDs[user.AccountID] || user.AccountID == selfAccountID {
						continue
					}

					if maxUsers > 0 && totalFetched >= maxUsers {
						capped = true
						break
					}

					seenAccountIDs[user.AccountID] = true
					newUsersThisBatch++
					totalFetched++

					if _, exists := userMap[user.AccountID]; !exists {
						userMap[user.AccountID] = user
					}
				}

				// Detect truncation and expand search
				truncated := len(result.users) >= 50 && newUsersThisBatch > 0
				status := "✓"
				if truncated {
					status = "⚠"
				}
				if capped {
					status = "⊗"
				}

				queryDisplay := result.query
				if queryDisplay == "" {
					queryDisplay = "(empty)"
				}

				fmt.Printf("[%d] %s Query: %s | Results: %4d | New: %3d | Total: %d",
					searchCount, status, queryDisplay, len(result.users), newUsersThisBatch, totalFetched)

				if maxUsers > 0 {
					fmt.Printf("/%d", maxUsers)
				}
				fmt.Printf(" | Pending: %d\n", pendingTasks)

				if truncated && customQuery == "" && !capped {
					select {
					case <-ctx.Done():
						return
					default:
						if maxUsers == 0 || totalFetched < maxUsers {
							alphabet := alphabet2
							if result.depth == 0 {
								alphabet = alphabet1
							}

							for _, char := range alphabet {
								newTask := userSearchTask{deskID: result.deskID, query: result.query + string(char), depth: result.depth + 1}
								pendingTasks++

								select {
								case <-ctx.Done():
									return
								case taskQueue <- newTask:
								}
							}
						}
					}
				}

				// Check if we're done
				if pendingTasks == 0 || capped {
					cancel()
					return
				}
			}
		}()

		// Start with initial query
		if customQuery != "" {
			taskQueue <- userSearchTask{deskID: desk.ID, query: customQuery, depth: 0}
		} else {
			taskQueue <- userSearchTask{deskID: desk.ID, query: "", depth: 0}
		}

		// Wait for completion or interruption
		go func() {
			<-ctx.Done()
			close(taskQueue)
		}()

		wg.Wait()
		close(results)
		processorWg.Wait()

		statusMsg := ""
		if capped {
			statusMsg = fmt.Sprintf(" [CAPPED at max=%d]", maxUsers)
		}
		fmt.Printf("  Found %d user(s) for this desk%s\n", totalFetched, statusMsg)
	}

	select {
	case <-interruptedChan:
		fmt.Println("\n*** Interrupted by user ***")
	default:
	}

	if len(userMap) == 0 {
		fmt.Println("\nNo users found")
		return nil
	}

	if outputPath != "" {
		return writeUsersToCSV(userMap, outputPath)
	}

	printUsers(userMap)
	return nil
}

func writeUsersToCSV(userMap map[string]User, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"AccountID", "DisplayName", "Email", "Avatar"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	for _, user := range userMap {
		avatar := user.Avatar
		if strings.Contains(avatar, defaultAvatar) {
			avatar = ""
		}
		if err := writer.Write([]string{user.AccountID, user.DisplayName, user.EmailAddress, avatar}); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}

	fmt.Printf("\nWrote %d users to %s\n", len(userMap), outputPath)
	return nil
}

func printUsers(userMap map[string]User) {
	fmt.Printf("\n\nUnique Users (%d):\n", len(userMap))
	fmt.Println(strings.Repeat("-", 100))

	for _, user := range userMap {
		fmt.Println("\nAccountID: " + user.AccountID)
		fmt.Println("  Name: " + user.DisplayName)
		if user.EmailAddress != "" {
			fmt.Println("  Email: " + user.EmailAddress)
		}
		if user.Avatar != "" && !strings.Contains(user.Avatar, defaultAvatar) {
			fmt.Println("  Avatar: " + user.Avatar)
		}
	}
}
