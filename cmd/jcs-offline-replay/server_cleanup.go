package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

func cmdServerCleanup(flags map[string]string, stdout io.Writer) error {
	runRecordPath := requireFlag(flags, "--run-record")
	if runRecordPath == "" {
		return fmt.Errorf("server-cleanup requires --run-record")
	}
	record, err := loadServerRunRecord(runRecordPath)
	if err != nil {
		return err
	}
	record.RunRecordPath = runRecordPath
	return runServerCleanup(record, stdout)
}

func runServerCleanup(record *serverRunRecord, stdout io.Writer) error {
	if record == nil {
		return fmt.Errorf("server cleanup record is nil")
	}
	if err := writef(stdout, "==> cleanup: %s\n", record.RunRecordPath); err != nil {
		return err
	}
	if record.DestroyStatus == serverRunStatusSucceeded {
		if err := writeServerAuditSummaries(*record); err != nil {
			return err
		}
		return writef(stdout, "cleanup already complete: %s\n", record.RunRecordPath)
	}
	record.DestroyStatus = serverRunStatusRunning
	if err := writeServerRunRecord(record.RunRecordPath, record); err != nil {
		return err
	}

	toolchain, err := resolveServerToolchain()
	if err != nil {
		record.DestroyStatus = serverRunStatusFailed
		record.LastError = err.Error()
		_ = writeServerRunRecord(record.RunRecordPath, record)
		return err
	}
	awsClients, err := newServerAWSClientsFunc(context.Background(), record.AWSRegion)
	if err != nil {
		record.DestroyStatus = serverRunStatusFailed
		record.LastError = err.Error()
		_ = writeServerRunRecord(record.RunRecordPath, record)
		return err
	}
	opts := serverEvidenceOptions{
		tag:          record.Tag,
		awsRegion:    record.AWSRegion,
		amiLockPath:  record.AMILockPath,
		lockFilePath: record.ProviderLockPath,
		infraDir:     record.InfraDir,
		root:         record.RepoRoot,
		state: serverStateConfig{
			Mode:      record.StateMode,
			Bucket:    record.StateBucket,
			Region:    record.StateRegion,
			LockTable: record.StateLockTable,
			Key:       record.StateKey,
		},
	}

	var errs []error
	bucketCtx, cancelBucket := cleanupContext(context.Background(), serverProvisionTimeout)
	if err := deleteStagingBucketFunc(bucketCtx, awsClients, record.StagingBucket); err != nil {
		errs = append(errs, err)
	}
	cancelBucket()

	if strings.TrimSpace(record.InfraDir) != "" {
		infraCtx, cancelInfra := cleanupContext(context.Background(), serverProvisionTimeout)
		if err := destroyServerInfrastructureFunc(infraCtx, opts, toolchain, record.SourceGitCommit, record.ProviderLockSHA256); err != nil {
			errs = append(errs, err)
		}
		cancelInfra()
	}

	if len(errs) != 0 {
		record.DestroyStatus = serverRunStatusFailed
		record.LastError = errors.Join(errs...).Error()
		record.CompletedAtUTC = manifestNowUTC().Format(time.RFC3339Nano)
		_ = writeServerRunRecord(record.RunRecordPath, record)
		return errors.Join(errs...)
	}
	record.DestroyStatus = serverRunStatusSucceeded
	record.LastError = ""
	if record.RunStatus == serverRunStatusRunning {
		record.RunStatus = serverRunStatusFailed
	}
	record.CompletedAtUTC = manifestNowUTC().Format(time.RFC3339Nano)
	if err := writeServerRunRecord(record.RunRecordPath, record); err != nil {
		return err
	}
	if err := writeServerAuditSummaries(*record); err != nil {
		return err
	}
	return writef(stdout, "cleanup complete: %s\n", record.RunRecordPath)
}
