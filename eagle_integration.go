package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type eagleAddRequest struct {
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Website    string   `json:"website,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Folders    []string `json:"folders,omitempty"`
	Annotation string   `json:"annotation,omitempty"`
}

func exportToEagle(record PhotoRecord) error {
	body, err := json.Marshal(buildEagleRequest(record))
	if err != nil {
		return err
	}
	url := strings.TrimRight(appConfig.Eagle.BaseURL, "/") + "/api/v2/item/add"
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTPステータス %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Eagle API応答の解析に失敗しました: %v", err)
	}
	if !strings.EqualFold(result.Status, "success") {
		if result.Message == "" {
			result.Message = "詳細なし"
		}
		return fmt.Errorf("Eagle APIエラー: %s", result.Message)
	}
	return nil
}

func buildEagleRequest(record PhotoRecord) eagleAddRequest {
	tags := []string{"VRChat", "type:" + string(record.SourceType)}
	if record.WorldName != "" && (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) {
		tags = append(tags, "wrld:"+record.WorldName)
	}
	if ym := shootMonth(record.ShootDate); ym != "" && (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) {
		tags = append(tags, ym)
	}
	if record.SourceType == SourceTypePhoto {
		for _, user := range record.PresentUsers {
			if strings.TrimSpace(user) != "" {
				tags = append(tags, "user:"+strings.TrimSpace(user))
			}
		}
	}
	folders := append([]string{}, appConfig.Eagle.Folders...)
	if appConfig.Eagle.FolderID != "" {
		folders = append(folders, appConfig.Eagle.FolderID)
	}
	req := eagleAddRequest{Path: record.SourcePath, Name: strings.TrimSuffix(filepath.Base(record.SourcePath), filepath.Ext(record.SourcePath)), Tags: tags, Folders: folders}
	if (record.SourceType == SourceTypePhoto || record.SourceType == SourceTypePrint) && record.WorldID != "" {
		req.Website = worldURLForID(record.WorldID)
		var lines []string
		if record.WorldName != "" {
			lines = append(lines, "World: "+record.WorldName)
		}
		if record.InstanceID != "" {
			lines = append(lines, "Instance: "+formatEagleInstanceLabel(record.InstanceID, record.InstanceType))
		}
		req.Annotation = strings.Join(lines, "\n")
	}
	return req
}

func formatEagleInstanceLabel(instanceID, instanceType string) string {
	label := instanceID
	if idx := strings.Index(instanceID, "~"); idx > 0 {
		label = instanceID[:idx]
	}
	typeLabel := strings.TrimSpace(instanceType)
	if typeLabel != "" {
		typeLabel = strings.ToUpper(typeLabel[:1]) + typeLabel[1:]
		if label != "" {
			return label + " (" + typeLabel + ")"
		}
		return typeLabel
	}
	return label
}

func testEagleConnection() error {
	url := strings.TrimRight(appConfig.Eagle.BaseURL, "/") + "/api/v2/app/info"
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTPステータス %s", resp.Status)
	}
	fmt.Printf("Eagle API 接続成功: %s\n", resp.Status)
	return nil
}
