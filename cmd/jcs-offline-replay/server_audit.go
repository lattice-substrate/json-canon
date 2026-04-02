package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type serverEvidenceSummary struct {
	SchemaVersion     string `json:"schema_version"`
	Tag               string `json:"tag"`
	SourceGitCommit   string `json:"source_git_commit"`
	SourceGitTag      string `json:"source_git_tag"`
	InfraManifestPath string `json:"infra_manifest_path"`
	InfraManifestSHA  string `json:"infra_manifest_sha256,omitempty"`
	X86EvidencePath   string `json:"x86_evidence_path"`
	X86EvidenceSHA    string `json:"x86_evidence_sha256,omitempty"`
	ArmEvidencePath   string `json:"arm64_evidence_path"`
	ArmEvidenceSHA    string `json:"arm64_evidence_sha256,omitempty"`
	X86BundlePath     string `json:"x86_bundle_path,omitempty"`
	ArmBundlePath     string `json:"arm64_bundle_path,omitempty"`
	WorkflowRunID     string `json:"workflow_run_id,omitempty"`
	WorkflowRunURL    string `json:"workflow_run_url,omitempty"`
	AWSAccountID      string `json:"aws_account_id,omitempty"`
	AWSRoleARN        string `json:"aws_role_arn,omitempty"`
	StateMode         string `json:"state_mode"`
	StateBucket       string `json:"state_bucket,omitempty"`
	StateRegion       string `json:"state_region,omitempty"`
	StateLockTable    string `json:"state_lock_table,omitempty"`
	StateKey          string `json:"state_key,omitempty"`
	DestroyStatus     string `json:"destroy_status"`
}

func writeServerAuditSummaries(record serverRunRecord) error {
	auditDir := filepath.Join(record.OutputDir, "audit")
	summary := serverEvidenceSummary{
		SchemaVersion:     "server-evidence-summary.v1",
		Tag:               record.Tag,
		SourceGitCommit:   record.SourceGitCommit,
		SourceGitTag:      record.SourceGitTag,
		InfraManifestPath: record.InfraManifestPath,
		X86EvidencePath:   record.X86EvidencePath,
		ArmEvidencePath:   record.ArmEvidencePath,
		X86BundlePath:     record.X86BundlePath,
		ArmBundlePath:     record.ArmBundlePath,
		WorkflowRunID:     record.WorkflowRunID,
		WorkflowRunURL:    record.WorkflowRunURL,
		AWSAccountID:      record.AWSAccountID,
		AWSRoleARN:        record.AWSRoleARN,
		StateMode:         record.StateMode,
		StateBucket:       record.StateBucket,
		StateRegion:       record.StateRegion,
		StateLockTable:    record.StateLockTable,
		StateKey:          record.StateKey,
		DestroyStatus:     record.DestroyStatus,
	}
	if strings.TrimSpace(record.InfraManifestPath) != "" {
		sha, err := fileSHA256(record.InfraManifestPath)
		if err != nil {
			return fmt.Errorf("sha256 infra manifest summary: %w", err)
		}
		summary.InfraManifestSHA = sha
	}
	for _, item := range []struct {
		path string
		dest *string
	}{
		{record.X86EvidencePath, &summary.X86EvidenceSHA},
		{record.ArmEvidencePath, &summary.ArmEvidenceSHA},
	} {
		if strings.TrimSpace(item.path) == "" {
			continue
		}
		sha, err := fileSHA256(item.path)
		if err != nil {
			return fmt.Errorf("sha256 evidence summary: %w", err)
		}
		*item.dest = sha
	}
	jsonPath := filepath.Join(auditDir, "server-evidence-summary.json")
	mdPath := filepath.Join(auditDir, "server-evidence-summary.md")
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server evidence summary: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteFile(jsonPath, data, filePerm); err != nil {
		return fmt.Errorf("write server evidence summary json: %w", err)
	}
	md := buildServerEvidenceSummaryMarkdown(summary)
	if err := atomicWriteFile(mdPath, []byte(md), filePerm); err != nil {
		return fmt.Errorf("write server evidence summary markdown: %w", err)
	}
	return nil
}

func buildServerEvidenceSummaryMarkdown(summary serverEvidenceSummary) string {
	lines := []string{
		"# Server Evidence Summary",
		"",
		"- tag: `" + summary.Tag + "`",
		"- source_git_commit: `" + summary.SourceGitCommit + "`",
		"- source_git_tag: `" + summary.SourceGitTag + "`",
		"- infra_manifest_sha256: `" + summary.InfraManifestSHA + "`",
		"- x86_64 evidence sha256: `" + summary.X86EvidenceSHA + "`",
		"- arm64 evidence sha256: `" + summary.ArmEvidenceSHA + "`",
		"- aws_account_id: `" + summary.AWSAccountID + "`",
		"- aws_role_arn: `" + summary.AWSRoleARN + "`",
		"- workflow_run_id: `" + summary.WorkflowRunID + "`",
		"- workflow_run_url: `" + summary.WorkflowRunURL + "`",
		"- state_mode: `" + summary.StateMode + "`",
		"- state_bucket: `" + summary.StateBucket + "`",
		"- state_region: `" + summary.StateRegion + "`",
		"- state_lock_table: `" + summary.StateLockTable + "`",
		"- state_key: `" + summary.StateKey + "`",
		"- destroy_status: `" + summary.DestroyStatus + "`",
		"",
		"## Paths",
		"",
		"- infra manifest: `" + summary.InfraManifestPath + "`",
		"- x86_64 evidence: `" + summary.X86EvidencePath + "`",
		"- arm64 evidence: `" + summary.ArmEvidencePath + "`",
		"- x86_64 bundle: `" + summary.X86BundlePath + "`",
		"- arm64 bundle: `" + summary.ArmBundlePath + "`",
		"",
	}
	return strings.Join(lines, "\n")
}
