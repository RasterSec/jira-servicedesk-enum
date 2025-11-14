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

type DocsGraphQLResponse struct {
	Data struct {
		HelpObjectStoreSearchArticles struct {
			TotalCount int `json:"totalCount"`
			Results    []struct {
				AbsoluteURL   string `json:"absoluteUrl"`
				ARI           string `json:"ari"`
				ContainerARI  string `json:"containerAri"`
				ContainerName string `json:"containerName"`
				Title         string `json:"title"`
			} `json:"results"`
		} `json:"helpObjectStore_searchArticles"`
	} `json:"data"`
	Errors []struct {
		Message string        `json:"message"`
		Path    []interface{} `json:"path"`
	} `json:"errors"`
}

type Document struct {
	ARI           string
	Title         string
	AbsoluteURL   string
	ContainerARI  string
	ContainerName string
}

type TenantInfo struct {
	CloudID string `json:"cloudId"`
}

type searchTask struct {
	query string
	depth int
}

type searchResult struct {
	query      string
	depth      int
	totalCount int
	docs       []Document
	err        error
}

func enumerateDocs(baseURL, cookie, alphabet1, alphabet2, output string, workers, timeout int) error {
	client := newClient(baseURL, cookie, time.Duration(timeout)*time.Second)

	cloudID, err := getCloudID(client)
	if err != nil {
		return fmt.Errorf("failed to get cloud ID: %w", err)
	}

	fmt.Println("Fetching all documents for cloud ID: " + cloudID)
	fmt.Println("Alphabet (layer 1): " + alphabet1)
	fmt.Println("Alphabet (layer 2+): " + alphabet2)
	fmt.Printf("Concurrent workers: %d\n", workers)
	fmt.Printf("Request timeout: %ds\n", timeout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interruptedChan := setupSignalHandler(cancel)

	docMap := make(map[string]Document)
	taskQueue := make(chan searchTask, 5000)
	results := make(chan searchResult, workers*2)

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

					totalCount, docs, err := searchDocuments(client, cloudID, task.query, 2147483647)

					select {
					case results <- searchResult{
						query:      task.query,
						depth:      task.depth,
						totalCount: totalCount,
						docs:       docs,
						err:        err,
					}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	// Results processor - single goroutine processes all results
	var processorWg sync.WaitGroup
	processorWg.Add(1)
	go func() {
		defer processorWg.Done()
		pendingTasks := 1
		searchCount := 0
		expectedTotal := 0

		for result := range results {
			pendingTasks--
			searchCount++

			if result.err != nil {
				fmt.Fprintf(os.Stderr, "Warning: search for '%s' failed: %v\n", result.query, result.err)
				if pendingTasks == 0 {
					cancel()
					return
				}
				continue
			}

			if searchCount == 1 {
				expectedTotal = result.totalCount
			}

			newDocs := 0
			for _, doc := range result.docs {
				if _, exists := docMap[doc.ARI]; !exists {
					docMap[doc.ARI] = doc
					newDocs++
				}
			}
			uniqueCount := len(docMap)

			truncated := len(result.docs) < result.totalCount
			status := "✓"
			if truncated {
				status = "⚠"
			}

			queryDisplay := result.query
			if queryDisplay == "" {
				queryDisplay = "(empty)"
			}

			fmt.Printf("[%d] %s Query: %s | Results: %4d/%d | New: %3d | Unique: %d/%d | Pending: %d\n",
				searchCount, status, queryDisplay, len(result.docs), result.totalCount, newDocs, uniqueCount, expectedTotal, pendingTasks)

			if truncated {
				select {
				case <-ctx.Done():
					return
				default:
					alphabet := alphabet2
					if result.depth == 0 {
						alphabet = alphabet1
					}

					for _, char := range alphabet {
						newTask := searchTask{query: result.query + string(char), depth: result.depth + 1}
						pendingTasks++

						select {
						case <-ctx.Done():
							return
						case taskQueue <- newTask:
						}
					}
				}
			}

			if pendingTasks == 0 || (uniqueCount >= expectedTotal && expectedTotal > 0) {
				cancel()
				return
			}
		}
	}()

	taskQueue <- searchTask{query: "", depth: 0}

	go func() {
		<-ctx.Done()
		close(taskQueue)
	}()

	wg.Wait()
	close(results)
	processorWg.Wait()

	select {
	case <-interruptedChan:
		fmt.Println("\n*** Interrupted by user ***")
	default:
	}

	fmt.Printf("\nTotal documents found: %d\n", len(docMap))

	if len(docMap) == 0 {
		fmt.Println("No documents found")
		return nil
	}

	// Convert map to slice
	finalDocs := make([]Document, 0, len(docMap))
	for _, doc := range docMap {
		finalDocs = append(finalDocs, doc)
	}

	if output != "" {
		return writeDocsToCSV(finalDocs, output)
	}

	printDocuments(finalDocs)
	return nil
}

func searchDocuments(client *Client, cloudID, queryTerm string, limit int) (int, []Document, error) {
	query := `query MyQuery($cloudId:ID!,$queryTerm:String,$limit:Int!){helpObjectStore_searchArticles(cloudId:$cloudId,queryTerm:$queryTerm,limit:$limit){...on HelpObjectStoreArticleSearchResults{totalCount results{absoluteUrl ari containerAri containerName title}}}}`

	variables := map[string]interface{}{
		"cloudId": cloudID,
		"limit":   limit,
	}

	if queryTerm != "" {
		variables["queryTerm"] = queryTerm
	}

	payload := map[string]interface{}{
		"query":         query,
		"operationName": "MyQuery",
		"variables":     variables,
	}

	resp, err := client.post("/gateway/api/graphql", payload)
	if err != nil {
		return 0, nil, fmt.Errorf("execute request: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := readBody(resp)
		return 0, nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response DocsGraphQLResponse
	if err := unmarshalJSON(resp, &response); err != nil {
		return 0, nil, fmt.Errorf("parse response: %w", err)
	}

	if len(response.Errors) > 0 {
		return 0, nil, fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	docs := make([]Document, 0, len(response.Data.HelpObjectStoreSearchArticles.Results))
	for _, result := range response.Data.HelpObjectStoreSearchArticles.Results {
		docs = append(docs, Document{
			ARI:           result.ARI,
			Title:         result.Title,
			AbsoluteURL:   result.AbsoluteURL,
			ContainerARI:  result.ContainerARI,
			ContainerName: result.ContainerName,
		})
	}

	return response.Data.HelpObjectStoreSearchArticles.TotalCount, docs, nil
}

func getCloudID(client *Client) (string, error) {
	resp, err := client.get("/_edge/tenant_info")
	if err != nil {
		return "", fmt.Errorf("get tenant info: %w", err)
	}

	var t TenantInfo
	if err := unmarshalJSON(resp, &t); err != nil {
		return "", fmt.Errorf("parse tenant info: %w", err)
	}

	return t.CloudID, nil
}

func writeDocsToCSV(docs []Document, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"ARI", "Title", "URL", "Container", "Container ARI"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	for _, doc := range docs {
		if err := writer.Write([]string{doc.ARI, doc.Title, doc.AbsoluteURL, doc.ContainerName, doc.ContainerARI}); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}

	fmt.Printf("\nWrote %d documents to %s\n", len(docs), outputPath)
	return nil
}

func printDocuments(docs []Document) {
	fmt.Println("\n\nDiscovered Documents:")
	fmt.Println(strings.Repeat("-", 100))

	for _, doc := range docs {
		fmt.Println("\nARI: " + doc.ARI)
		fmt.Println("  Title: " + doc.Title)
		fmt.Println("  URL: " + doc.AbsoluteURL)
		fmt.Println("  Container: " + doc.ContainerName + " (" + doc.ContainerARI + ")")
	}
}
