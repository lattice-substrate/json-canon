package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	serverSSMReadyTimeout       = 8 * time.Minute
	serverSSMCommandPoll        = 2 * time.Second
	serverSSMCommandTimeoutSecs = 1800
	serverPresignExpiry         = 20 * time.Minute
)

type serverAWSClients struct {
	config    aws.Config
	ec2       *ec2.Client
	s3        *s3.Client
	s3Presign *s3.PresignClient
	ssm       *ssm.Client
}

type serverStaging struct {
	bucket string
	x86    stagedServerArtifacts
	arm    stagedServerArtifacts
}

type stagedServerArtifacts struct {
	bundleKey string
	workerKey string
}

type awsReleaseHostSelector struct {
	HostID          string `json:"host_id"`
	NodeID          string `json:"node_id"`
	Architecture    string `json:"architecture"`
	Distro          string `json:"distro"`
	KernelFamily    string `json:"kernel_family"`
	InstanceType    string `json:"instance_type"`
	AMIOwner        string `json:"ami_owner,omitempty"`
	AMIName         string `json:"ami_name,omitempty"`
	AMISource       string `json:"ami_source,omitempty"`
	AMISSMParameter string `json:"ami_ssm_parameter,omitempty"`
}

type awsReleaseHostCatalog struct {
	SchemaVersion string                   `json:"schema_version"`
	Hosts         []awsReleaseHostSelector `json:"hosts"`
}

type awsReleaseHostLock struct {
	SchemaVersion  string                   `json:"schema_version"`
	GeneratedAtUTC string                   `json:"generated_at_utc"`
	AWSRegion      string                   `json:"aws_region"`
	Hosts          []awsReleaseHostLockItem `json:"hosts"`
}

type awsReleaseHostLockItem struct {
	HostID          string `json:"host_id"`
	NodeID          string `json:"node_id"`
	Architecture    string `json:"architecture"`
	Distro          string `json:"distro"`
	KernelFamily    string `json:"kernel_family"`
	InstanceType    string `json:"instance_type"`
	AMIID           string `json:"ami_id"`
	AMIOwner        string `json:"ami_owner,omitempty"`
	AMIName         string `json:"ami_name,omitempty"`
	AMISource       string `json:"ami_source,omitempty"`
	AMISSMParameter string `json:"ami_ssm_parameter,omitempty"`
}

func newServerAWSClients(ctx context.Context, region string) (serverAWSClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return serverAWSClients{}, fmt.Errorf("load aws sdk config: %w", err)
	}
	s3Client := s3.NewFromConfig(cfg)
	return serverAWSClients{
		config:    cfg,
		ec2:       ec2.NewFromConfig(cfg),
		s3:        s3Client,
		s3Presign: s3.NewPresignClient(s3Client),
		ssm:       ssm.NewFromConfig(cfg),
	}, nil
}

func createStagingBucket(ctx context.Context, clients serverAWSClients, tag string) (string, error) {
	bucket := fmt.Sprintf("jcs-offline-%s-%s", sanitizeBucketToken(tag), randomBucketSuffix())
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}
	if region := strings.TrimSpace(clients.config.Region); region != "" && region != "us-east-1" {
		input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		}
	}
	if _, err := clients.s3.CreateBucket(ctx, input); err != nil {
		return "", fmt.Errorf("create staging bucket %s: %w", bucket, err)
	}
	return bucket, nil
}

func deleteStagingBucket(ctx context.Context, clients serverAWSClients, bucket string) error {
	if strings.TrimSpace(bucket) == "" {
		return nil
	}
	paginator := s3.NewListObjectsV2Paginator(clients.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list staging bucket objects %s: %w", bucket, err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		objects := make([]s3types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			objects = append(objects, s3types.ObjectIdentifier{Key: obj.Key})
		}
		if len(objects) == 0 {
			continue
		}
		if _, err := clients.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3types.Delete{Objects: objects},
		}); err != nil {
			return fmt.Errorf("delete staging bucket objects %s: %w", bucket, err)
		}
	}
	if _, err := clients.s3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		return fmt.Errorf("delete staging bucket %s: %w", bucket, err)
	}
	return nil
}

func uploadStagingFile(ctx context.Context, clients serverAWSClients, bucket, key, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open staging artifact %s: %w", path, err)
	}
	defer closeBestEffort(file)
	if _, err := clients.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	}); err != nil {
		return fmt.Errorf("upload staging artifact %s to s3://%s/%s: %w", path, bucket, key, err)
	}
	return nil
}

func presignGetObjectURL(ctx context.Context, clients serverAWSClients, bucket, key string) (string, error) {
	req, err := clients.s3Presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(serverPresignExpiry))
	if err != nil {
		return "", fmt.Errorf("presign get object s3://%s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}

func presignPutObjectURL(ctx context.Context, clients serverAWSClients, bucket, key string) (string, error) {
	req, err := clients.s3Presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(serverPresignExpiry))
	if err != nil {
		return "", fmt.Errorf("presign put object s3://%s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}

func downloadStagingObject(ctx context.Context, clients serverAWSClients, bucket, key string) ([]byte, error) {
	resp, err := clients.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get staging object s3://%s/%s: %w", bucket, key, err)
	}
	defer closeBestEffort(resp.Body)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read staging object s3://%s/%s: %w", bucket, key, err)
	}
	return data, nil
}

func waitForSSMManagedInstances(ctx context.Context, clients serverAWSClients, hosts map[string]provisionedHost, timeout time.Duration) error {
	instanceIDs := make([]string, 0, len(hosts))
	for _, hostID := range sortedProvisionedHostIDs(hosts) {
		if id := strings.TrimSpace(hosts[hostID].InstanceID); id != "" {
			instanceIDs = append(instanceIDs, id)
		}
	}
	if len(instanceIDs) == 0 {
		return fmt.Errorf("no provisioned instance ids available for ssm wait")
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(serverSSMCommandPoll)
	defer ticker.Stop()
	for {
		online, err := describeOnlineInstanceIDs(deadlineCtx, clients, instanceIDs)
		if err == nil && len(online) == len(instanceIDs) {
			return nil
		}
		select {
		case <-deadlineCtx.Done():
			if err != nil {
				return fmt.Errorf("wait for ssm managed instances: %w", err)
			}
			return fmt.Errorf("ssm unavailable after %s", timeout)
		case <-ticker.C:
		}
	}
}

func describeOnlineInstanceIDs(ctx context.Context, clients serverAWSClients, instanceIDs []string) (map[string]struct{}, error) {
	out, err := clients.ssm.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    aws.String("InstanceIds"),
				Values: instanceIDs,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	online := make(map[string]struct{}, len(out.InstanceInformationList))
	for _, info := range out.InstanceInformationList {
		if info.InstanceId == nil {
			continue
		}
		if info.PingStatus == ssmtypes.PingStatusOnline {
			online[*info.InstanceId] = struct{}{}
		}
	}
	return online, nil
}

func runSSMShellScript(ctx context.Context, clients serverAWSClients, instanceID, comment, script string, timeout time.Duration) (string, error) {
	timeoutSeconds := int32(timeout / time.Second)
	if timeoutSeconds <= 0 {
		timeoutSeconds = serverSSMCommandTimeoutSecs
	}
	sendOut, err := clients.ssm.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String(comment),
		InstanceIds:  []string{instanceID},
		Parameters: map[string][]string{
			"commands": {script},
		},
		TimeoutSeconds: aws.Int32(timeoutSeconds),
		MaxConcurrency: aws.String("1"),
		MaxErrors:      aws.String("0"),
	})
	if err != nil {
		return "", fmt.Errorf("send ssm command to %s: %w", instanceID, err)
	}
	if sendOut.Command == nil || sendOut.Command.CommandId == nil {
		return "", fmt.Errorf("send ssm command to %s: missing command id", instanceID)
	}
	commandID := *sendOut.Command.CommandId
	ticker := time.NewTicker(serverSSMCommandPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
		out, err := clients.ssm.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  aws.String(commandID),
			InstanceId: aws.String(instanceID),
		})
		if err != nil {
			if strings.Contains(err.Error(), "InvocationDoesNotExist") {
				continue
			}
			return "", fmt.Errorf("get ssm command invocation %s for %s: %w", commandID, instanceID, err)
		}
		switch out.Status {
		case ssmtypes.CommandInvocationStatusPending, ssmtypes.CommandInvocationStatusInProgress, ssmtypes.CommandInvocationStatusDelayed:
			continue
		case ssmtypes.CommandInvocationStatusSuccess:
			return aws.ToString(out.StandardOutputContent), nil
		default:
			stderr := strings.TrimSpace(aws.ToString(out.StandardErrorContent))
			stdout := strings.TrimSpace(aws.ToString(out.StandardOutputContent))
			if stderr != "" {
				return "", fmt.Errorf("ssm command %s on %s failed (%s): %s", commandID, instanceID, out.Status, stderr)
			}
			if stdout != "" {
				return "", fmt.Errorf("ssm command %s on %s failed (%s): %s", commandID, instanceID, out.Status, stdout)
			}
			return "", fmt.Errorf("ssm command %s on %s failed (%s)", commandID, instanceID, out.Status)
		}
	}
}

func sanitizeBucketToken(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "v")
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	token := strings.Trim(b.String(), "-")
	if token == "" {
		return "release"
	}
	return token
}

func randomBucketSuffix() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var out strings.Builder
	for i := 0; i < 10; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "fallback1234"
		}
		out.WriteByte(alphabet[n.Int64()])
	}
	return out.String()
}

func cmdRefreshAWSAMILock(flags map[string]string, stdout io.Writer) error {
	root := resolveRepoRoot()
	inputPath := resolveServerEvidencePath(root, flags["--input"], filepath.Join("infra", "aws_release_hosts.json"))
	outputPath := resolveServerEvidencePath(root, flags["--output"], filepath.Join("infra", "aws_release_hosts.lock.json"))
	region := defaultString(flags, "--aws-region", defaultAWSRegion)
	catalog, err := loadAWSReleaseHostCatalog(inputPath)
	if err != nil {
		return err
	}
	clients, err := newServerAWSClients(context.Background(), region)
	if err != nil {
		return err
	}
	lock, err := resolveAWSReleaseHostLock(context.Background(), clients, catalog, region)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("encode aws ami lock: %w", err)
	}
	data = append(data, '\n')
	if err := atomicWriteFile(outputPath, data, filePerm); err != nil {
		return fmt.Errorf("write aws ami lock: %w", err)
	}
	return writef(stdout, "aws ami lock: %s\n", outputPath)
}

func loadAWSReleaseHostCatalog(path string) (*awsReleaseHostCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read aws release host catalog: %w", err)
	}
	var catalog awsReleaseHostCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode aws release host catalog: %w", err)
	}
	if catalog.SchemaVersion != "aws-release-hosts.v1" {
		return nil, fmt.Errorf("unsupported aws release host catalog schema_version %q", catalog.SchemaVersion)
	}
	if len(catalog.Hosts) == 0 {
		return nil, fmt.Errorf("aws release host catalog is empty")
	}
	return &catalog, nil
}

func resolveAWSReleaseHostLock(ctx context.Context, clients serverAWSClients, catalog *awsReleaseHostCatalog, region string) (*awsReleaseHostLock, error) {
	if catalog == nil {
		return nil, fmt.Errorf("aws release host catalog is nil")
	}
	items := make([]awsReleaseHostLockItem, 0, len(catalog.Hosts))
	for _, host := range catalog.Hosts {
		amiID, err := resolveAMIIDForHost(ctx, clients, host)
		if err != nil {
			return nil, err
		}
		items = append(items, awsReleaseHostLockItem{
			HostID:          host.HostID,
			NodeID:          host.NodeID,
			Architecture:    host.Architecture,
			Distro:          host.Distro,
			KernelFamily:    host.KernelFamily,
			InstanceType:    host.InstanceType,
			AMIID:           amiID,
			AMIOwner:        host.AMIOwner,
			AMIName:         host.AMIName,
			AMISource:       host.AMISource,
			AMISSMParameter: host.AMISSMParameter,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].HostID < items[j].HostID
	})
	return &awsReleaseHostLock{
		SchemaVersion:  "aws-release-host-lock.v1",
		GeneratedAtUTC: manifestNowUTC().Format(time.RFC3339Nano),
		AWSRegion:      region,
		Hosts:          items,
	}, nil
}

func resolveAMIIDForHost(ctx context.Context, clients serverAWSClients, host awsReleaseHostSelector) (string, error) {
	if strings.EqualFold(strings.TrimSpace(host.AMISource), "ssm") {
		out, err := clients.ssm.GetParameter(ctx, &ssm.GetParameterInput{
			Name: aws.String(host.AMISSMParameter),
		})
		if err != nil {
			return "", fmt.Errorf("resolve ami for %s from ssm parameter %s: %w", host.HostID, host.AMISSMParameter, err)
		}
		return strings.TrimSpace(aws.ToString(out.Parameter.Value)), nil
	}
	out, err := clients.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{host.AMIOwner},
		Filters: []ec2types.Filter{
			{Name: aws.String("name"), Values: []string{host.AMIName}},
			{Name: aws.String("architecture"), Values: []string{host.Architecture}},
			{Name: aws.String("virtualization-type"), Values: []string{"hvm"}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("resolve ami for %s from image selectors: %w", host.HostID, err)
	}
	if len(out.Images) == 0 {
		return "", fmt.Errorf("resolve ami for %s: no images found", host.HostID)
	}
	sort.Slice(out.Images, func(i, j int) bool {
		return aws.ToString(out.Images[i].CreationDate) > aws.ToString(out.Images[j].CreationDate)
	})
	return strings.TrimSpace(aws.ToString(out.Images[0].ImageId)), nil
}
