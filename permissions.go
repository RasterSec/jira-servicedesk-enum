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
	"fmt"
	"strings"
	"time"
)

type PermissionsResponse struct {
	Permissions map[string]Permission `json:"permissions"`
}

type Permission struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type MyPermissionsResponse struct {
	Permissions map[string]MyPermission `json:"permissions"`
}

type MyPermission struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Description    string `json:"description"`
	HavePermission bool   `json:"havePermission"`
}

func checkPermissions(baseURL, cookie string) error {
	client := newClient(baseURL, "", 10*time.Second)

	resp, err := client.get("/rest/api/3/permissions")
	if err != nil {
		return fmt.Errorf("get permissions list: %w", err)
	}

	var permsResp PermissionsResponse
	if err := unmarshalJSON(resp, &permsResp); err != nil {
		return fmt.Errorf("parse permissions: %w", err)
	}

	permKeys := make([]string, 0, len(permsResp.Permissions))
	for key := range permsResp.Permissions {
		permKeys = append(permKeys, key)
	}

	client.cookie = cookie
	queryString := strings.Join(permKeys, ",")
	resp, err = client.get("/rest/api/3/mypermissions?permissions=" + queryString)
	if err != nil {
		return fmt.Errorf("get my permissions: %w", err)
	}

	var myPermsResp MyPermissionsResponse
	if err := unmarshalJSON(resp, &myPermsResp); err != nil {
		return fmt.Errorf("parse my permissions: %w", err)
	}

	fmt.Println("\nPermissions:")
	fmt.Println(strings.Repeat("-", 80))

	for _, perm := range myPermsResp.Permissions {
		status := "✗"
		if perm.HavePermission {
			status = "✓"
		}
		fmt.Printf("[%s] %-30s %-15s %s\n", status, perm.Name, "("+perm.Type+")", perm.Key)
	}

	return nil
}
