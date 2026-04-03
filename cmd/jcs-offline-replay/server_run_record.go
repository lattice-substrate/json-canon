package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	serverRunRecordSchemaVersion = "server-run.v1"

	serverRunStatusPending   = "pending"
	serverRunStatusRunning   = "running"
	serverRunStatusSucceeded = "succeeded"
	serverRunStatusFailed    = "failed"

	serverStateModeLocal  = "local"
	serverStateModeRemote = "remote"
)

type serverRunRecord struct {
	SchemaVersion            string `json:"schema_version"`
	Tag                      string `json:"tag"`
	SourceGitCommit          string `json:"source_git_commit"`
	SourceGitTag             string `json:"source_git_tag"`
	RepoRoot                 string `json:"repo_root"`
	SourceRoot               string `json:"source_root"`
	OutputDir                string `json:"output_dir"`
	RunRecordPath            string `json:"run_record_path"`
	AWSRegion                string `json:"aws_region"`
	StateMode                string `json:"state_mode"`
	StateBucket              string `json:"state_bucket,omitempty"`
	StateRegion              string `json:"state_region,omitempty"`
	StateLockTable           string `json:"state_lock_table,omitempty"`
	StateKey                 string `json:"state_key,omitempty"`
	InfraDir                 string `json:"infra_dir"`
	ProviderLockPath         string `json:"provider_lock_path"`
	ProviderLockSHA256       string `json:"provider_lock_sha256"`
	AMILockPath              string `json:"ami_lock_path"`
	StagingBucket            string `json:"staging_bucket,omitempty"`
	InfraManifestPath        string `json:"infra_manifest_path,omitempty"`
	CrossArchCompareJSONPath string `json:"cross_arch_compare_json_path,omitempty"`
	CrossArchCompareMDPath   string `json:"cross_arch_compare_markdown_path,omitempty"`
	X86EvidencePath          string `json:"x86_evidence_path,omitempty"`
	ArmEvidencePath          string `json:"arm64_evidence_path,omitempty"`
	X86BundlePath            string `json:"x86_bundle_path,omitempty"`
	ArmBundlePath            string `json:"arm64_bundle_path,omitempty"`
	X86ControlPath           string `json:"x86_control_binary_path,omitempty"`
	ArmControlPath           string `json:"arm64_control_binary_path,omitempty"`
	WorkflowRunID            string `json:"workflow_run_id,omitempty"`
	WorkflowRunURL           string `json:"workflow_run_url,omitempty"`
	AWSAccountID             string `json:"aws_account_id,omitempty"`
	AWSRoleARN               string `json:"aws_role_arn,omitempty"`
	ProvisionStatus          string `json:"provision_status"`
	StagingStatus            string `json:"staging_status"`
	DiscoveryStatus          string `json:"discovery_status"`
	InfraManifestStatus      string `json:"infra_manifest_status"`
	X86ReplayStatus          string `json:"x86_replay_status"`
	ArmReplayStatus          string `json:"arm64_replay_status"`
	X86GateStatus            string `json:"x86_gate_status"`
	ArmGateStatus            string `json:"arm64_gate_status"`
	CrossArchStatus          string `json:"cross_arch_status"`
	DestroyStatus            string `json:"destroy_status"`
	RunStatus                string `json:"run_status"`
	LastError                string `json:"last_error,omitempty"`
	StartedAtUTC             string `json:"started_at_utc"`
	CompletedAtUTC           string `json:"completed_at_utc,omitempty"`
}

func newServerRunRecord(path string, runtimeOpts, cleanupOpts serverEvidenceOptions, gitCommit, sourceRoot, lockSHA string) serverRunRecord {
	runID := lookupEnvTrimmed("GITHUB_RUN_ID")
	record := serverRunRecord{
		SchemaVersion:       serverRunRecordSchemaVersion,
		Tag:                 runtimeOpts.tag,
		SourceGitCommit:     gitCommit,
		SourceGitTag:        runtimeOpts.tag,
		RepoRoot:            cleanupOpts.root,
		SourceRoot:          sourceRoot,
		OutputDir:           runtimeOpts.outputDir,
		RunRecordPath:       path,
		AWSRegion:           runtimeOpts.awsRegion,
		StateMode:           runtimeOpts.state.Mode,
		StateBucket:         runtimeOpts.state.Bucket,
		StateRegion:         runtimeOpts.state.Region,
		StateLockTable:      runtimeOpts.state.LockTable,
		StateKey:            runtimeOpts.state.Key,
		InfraDir:            cleanupOpts.infraDir,
		ProviderLockPath:    cleanupOpts.lockFilePath,
		ProviderLockSHA256:  lockSHA,
		AMILockPath:         cleanupOpts.amiLockPath,
		X86EvidencePath:     filepath.Join(runtimeOpts.outputDir, "x86_64", "offline-evidence.json"),
		ArmEvidencePath:     filepath.Join(runtimeOpts.outputDir, "arm64", "offline-evidence.json"),
		StartedAtUTC:        manifestNowUTC().Format(time.RFC3339Nano),
		ProvisionStatus:     serverRunStatusPending,
		StagingStatus:       serverRunStatusPending,
		DiscoveryStatus:     serverRunStatusPending,
		InfraManifestStatus: serverRunStatusPending,
		X86ReplayStatus:     serverRunStatusPending,
		ArmReplayStatus:     serverRunStatusPending,
		X86GateStatus:       serverRunStatusPending,
		ArmGateStatus:       serverRunStatusPending,
		CrossArchStatus:     serverRunStatusPending,
		DestroyStatus:       serverRunStatusPending,
		RunStatus:           serverRunStatusRunning,
		WorkflowRunID:       runID,
	}
	if runID != "" {
		record.WorkflowRunURL = serverRepoURL + "/actions/runs/" + runID
	}
	return record
}

func loadServerRunRecord(path string) (*serverRunRecord, error) {
	//nolint:gosec // REQ:OFFLINE-AUTO-001 cleanup and audit flows load explicit run-record paths.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server run record: %w", err)
	}
	var record serverRunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode server run record: %w", err)
	}
	if record.SchemaVersion != serverRunRecordSchemaVersion {
		return nil, fmt.Errorf("unsupported server run record schema_version %q", record.SchemaVersion)
	}
	return &record, nil
}

func writeServerRunRecord(path string, record *serverRunRecord) error {
	if record == nil {
		return fmt.Errorf("server run record is nil")
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server run record: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("write server run record: %w", err)
	}
	return nil
}

func (r *serverEvidenceRuntime) persistRunRecord() error {
	if strings.TrimSpace(r.runRecordPath) == "" {
		return nil
	}
	return writeServerRunRecord(r.runRecordPath, &r.runRecord)
}

func (r *serverEvidenceRuntime) setRunRecordStatus(field *string, status string) error {
	*field = status
	return r.persistRunRecord()
}

func (r *serverEvidenceRuntime) failRunRecord(err error) error {
	if err == nil {
		return nil
	}
	r.runRecord.RunStatus = serverRunStatusFailed
	r.runRecord.LastError = err.Error()
	return r.persistRunRecord()
}

func (r *serverEvidenceRuntime) completeRunRecordSuccess() error {
	r.runRecord.RunStatus = serverRunStatusSucceeded
	r.runRecord.LastError = ""
	r.runRecord.CompletedAtUTC = manifestNowUTC().Format(time.RFC3339Nano)
	return r.persistRunRecord()
}

func (r *serverEvidenceRuntime) completeRunRecordFailure(err error) error {
	r.runRecord.RunStatus = serverRunStatusFailed
	if err != nil {
		r.runRecord.LastError = err.Error()
	}
	r.runRecord.CompletedAtUTC = manifestNowUTC().Format(time.RFC3339Nano)
	return r.persistRunRecord()
}
