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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// GraphQL response structures for document search
type DocsGraphQLResponse struct {
	Data struct {
		HelpObjectStoreSearchArticles struct {
			Results []struct {
				ARI         string `json:"ari"`
				Title       string `json:"title"`
				DisplayLink string `json:"displayLink"`
				Metadata    struct {
					SearchStrategy string `json:"searchStrategy"`
					IsExternal     bool   `json:"isExternal"`
				} `json:"metadata"`
				SourceSystem string `json:"sourceSystem"`
			} `json:"results"`
			TotalCount int `json:"totalCount"`
		} `json:"helpObjectStore_searchArticles"`
	} `json:"data"`
}

type AccessibleResource struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type Document struct {
	ARI          string
	Title        string
	DisplayLink  string
	FullURL      string
	SourceSystem string
	IsExternal   bool
}

type TenantInfo struct {
	CloudID string `json:"cloudId"`
}

func enumerateDocs(baseURL, cookie string, searchTerm string, limit int, alphabet, output string) error {
	fmt.Println("Extracting cloud ID from API")

	cloudID, err := getCloudIDFromAPI(baseURL, cookie)
	if err != nil {
		return fmt.Errorf("failed to get cloud ID: %w", err)
	}
	fmt.Printf("Extracted Cloud ID: %s\n\n", cloudID)

	docMap := make(map[string]Document)
	totalSearched := 0

	// Build search queries
	var queries []string
	if searchTerm != "" {
		queries = []string{searchTerm}
	} else {
		// Start with single characters from alphabet
		for _, char := range alphabet {
			queries = append(queries, string(char))
		}
	}

	fmt.Printf("Starting document enumeration with %d initial queries...\n\n", len(queries))

	for len(queries) > 0 {
		query := queries[0]
		queries = queries[1:]

		totalSearched++

		docs, total, err := searchDocuments(baseURL, cloudID, cookie, query, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching (query=%s): %v\n", query, err)
			continue
		}

		newDocs := 0
		for _, doc := range docs {
			if _, exists := docMap[doc.ARI]; !exists {
				docMap[doc.ARI] = doc
				newDocs++
			}
		}

		fmt.Printf("[%d] Query: %-30s | Found: %4d total (%d new) | Unique docs: %d\n",
			totalSearched, truncateString(query, 30), total, newDocs, len(docMap))
	}

	fmt.Printf("\n\nTotal unique documents found: %d\n", len(docMap))

	if len(docMap) == 0 {
		fmt.Println("No documents found")
		return nil
	}

	if output != "" {
		return writeDocsToCSV(docMap, output)
	}

	printDocuments(docMap)
	return nil
}

func searchDocuments(baseURL, cloudID, cookie, searchTerm string, limit int) ([]Document, int, error) {
	graphqlHash := "2b7701d308127724d13b7beb2b1c606c10713f908a96b13c96db5350d5ecc6ef"
	endpoint := fmt.Sprintf("%s/gateway/api/graphql/pq/%s", baseURL, graphqlHash)

	payload := map[string]interface{}{
		"variables": map[string]interface{}{
			"cloudId":               cloudID,
			"queryTerm":             searchTerm,
			"portalIds":             []string{},
			"categoryIds":           []string{},
			"limit":                 limit,
			"highlight":             false,
			"isSourceSystemEnabled": true,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", fmt.Sprintf("customer.account.session.token=%s", cookie))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var response DocsGraphQLResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, 0, fmt.Errorf("parse response: %w", err)
	}

	docs := make([]Document, 0, len(response.Data.HelpObjectStoreSearchArticles.Results))
	for _, result := range response.Data.HelpObjectStoreSearchArticles.Results {
		docs = append(docs, Document{
			ARI:          result.ARI,
			Title:        result.Title,
			DisplayLink:  result.DisplayLink,
			FullURL:      baseURL + result.DisplayLink,
			SourceSystem: result.SourceSystem,
			IsExternal:   result.Metadata.IsExternal,
		})
	}

	return docs, response.Data.HelpObjectStoreSearchArticles.TotalCount, nil
}

func getCloudIDFromAPI(baseURL, cookie string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", baseURL+"/_edge/tenant_info", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Cookie", fmt.Sprintf("customer.account.session.token=%s", cookie))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var t TenantInfo
	if err := json.Unmarshal(body, &t); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return t.CloudID, nil
}

func writeDocsToCSV(docMap map[string]Document, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"Title", "URL", "ARI", "Source", "IsExternal"}); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	for _, doc := range docMap {
		isExternal := "false"
		if doc.IsExternal {
			isExternal = "true"
		}
		if err := writer.Write([]string{doc.Title, doc.FullURL, doc.ARI, doc.SourceSystem, isExternal}); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}

	fmt.Printf("\nWrote %d documents to %s\n", len(docMap), outputPath)
	return nil
}

func printDocuments(docMap map[string]Document) {
	fmt.Printf("\n\nDiscovered Documents:\n")
	fmt.Println(strings.Repeat("-", 100))

	for _, doc := range docMap {
		fmt.Printf("\nTitle: %s\n", doc.Title)
		fmt.Printf("  URL: %s\n", doc.FullURL)
		fmt.Printf("  Source: %s\n", doc.SourceSystem)
		if doc.IsExternal {
			fmt.Printf("  External: true\n")
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
