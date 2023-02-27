/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vfs

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"k8s.io/klog/v2"

	"k8s.io/kops/upup/pkg/fi/cloudup/terraformWriter"
	"k8s.io/kops/util/pkg/hashing"
)

type S3Path struct {
	s3Context *S3Context
	bucket    string
	key       string
	etag      *string

	// scheme is configurable in case an S3 compatible custom
	// endpoint is specified
	scheme string
	// sse specifies if server side encryption should be enabled
	sse bool
}

var (
	_ Path          = &S3Path{}
	_ TerraformPath = &S3Path{}
	_ HasHash       = &S3Path{}
)

// S3Acl is an ACL implementation for objects on S3
type S3Acl struct {
	RequestACL *string
}

func newS3Path(s3Context *S3Context, scheme string, bucket string, key string, sse bool) *S3Path {
	bucket = strings.TrimSuffix(bucket, "/")
	key = strings.TrimPrefix(key, "/")

	return &S3Path{
		s3Context: s3Context,
		bucket:    bucket,
		key:       key,
		scheme:    scheme,
		sse:       sse,
	}
}

func (p *S3Path) Path() string {
	return p.scheme + "://" + p.bucket + "/" + p.key
}

func (p *S3Path) Bucket() string {
	return p.bucket
}

func (p *S3Path) Key() string {
	return p.key
}

func (p *S3Path) String() string {
	return p.Path()
}

func (p *S3Path) Remove() error {
	ctx := context.TODO()

	client, err := p.client(ctx)
	if err != nil {
		return err
	}

	klog.V(8).Infof("removing file %s", p)

	request := &s3.DeleteObjectInput{}
	request.Bucket = aws.String(p.bucket)
	request.Key = aws.String(p.key)

	_, err = client.DeleteObjectWithContext(ctx, request)
	if err != nil {
		// TODO: Check for not-exists, return os.NotExist

		return fmt.Errorf("error deleting %s: %v", p, err)
	}

	return nil
}

func (p *S3Path) RemoveAllVersions() error {
	ctx := context.TODO()

	client, err := p.client(ctx)
	if err != nil {
		return err
	}

	klog.V(8).Infof("removing all versions of file %s", p)

	request := &s3.ListObjectVersionsInput{
		Bucket: aws.String(p.bucket),
		Prefix: aws.String(p.key),
	}

	var versions []*s3.ObjectVersion
	var deleteMarkers []*s3.DeleteMarkerEntry
	if err := client.ListObjectVersionsPagesWithContext(ctx, request, func(page *s3.ListObjectVersionsOutput, lastPage bool) bool {
		versions = append(versions, page.Versions...)
		deleteMarkers = append(deleteMarkers, page.DeleteMarkers...)
		return true
	}); err != nil {
		return fmt.Errorf("error listing all versions of file %s: %v", p, err)
	}

	if len(versions) == 0 && len(deleteMarkers) == 0 {
		return os.ErrNotExist
	}

	var objects []*s3.ObjectIdentifier
	for _, version := range versions {
		klog.V(8).Infof("removing file %s version %q", p, aws.StringValue(version.VersionId))
		file := s3.ObjectIdentifier{
			Key:       version.Key,
			VersionId: version.VersionId,
		}
		objects = append(objects, &file)
	}
	for _, version := range deleteMarkers {
		klog.V(8).Infof("removing marker %s version %q", p, aws.StringValue(version.VersionId))
		marker := s3.ObjectIdentifier{
			Key:       version.Key,
			VersionId: version.VersionId,
		}
		objects = append(objects, &marker)
	}

	for len(objects) > 0 {
		request := &s3.DeleteObjectsInput{
			Bucket: aws.String(p.bucket),
			Delete: &s3.Delete{},
		}

		// DeleteObjects can only process 1000 objects per call
		if len(objects) > 1000 {
			request.Delete.Objects = objects[:1000]
			objects = objects[1000:]
		} else {
			request.Delete.Objects = objects
			objects = nil
		}

		klog.V(8).Infof("removing %d file/marker versions\n", len(request.Delete.Objects))

		_, err = client.DeleteObjectsWithContext(ctx, request)
		if err != nil {
			return fmt.Errorf("error removing %d file/marker versions: %v", len(request.Delete.Objects), err)
		}
	}

	return nil
}

func (p *S3Path) Join(relativePath ...string) Path {
	args := []string{p.key}
	args = append(args, relativePath...)
	joined := path.Join(args...)
	return &S3Path{
		s3Context: p.s3Context,
		bucket:    p.bucket,
		key:       joined,
		scheme:    p.scheme,
		sse:       p.sse,
	}
}

func (p *S3Path) getServerSideEncryption(ctx context.Context) (sse *string, sseLog string, err error) {
	// If we are on an S3 implementation that supports SSE (i.e. not
	// DO), we use server-side-encryption, it doesn't really cost us
	// anything.  But if the bucket has a defaultEncryption policy
	// instead, we honor that - it is likely to be a higher encryption
	// standard.
	sseLog = "-"
	if p.sse {
		bucketDetails, err := p.getBucketDetails(ctx)
		if err != nil {
			return nil, "", err
		}
		defaultEncryption := bucketDetails.hasServerSideEncryptionByDefault(ctx)
		if defaultEncryption {
			sseLog = "DefaultBucketEncryption"
		} else {
			sseLog = "AES256"
			sse = aws.String("AES256")
		}
	}

	return sse, sseLog, nil
}

func (p *S3Path) getRequestACL(aclObj ACL) (*string, error) {
	acl := os.Getenv("KOPS_STATE_S3_ACL")
	acl = strings.TrimSpace(acl)
	if acl != "" {
		klog.V(8).Infof("Using KOPS_STATE_S3_ACL=%s", acl)
		return &acl, nil
	} else if aclObj != nil {
		s3Acl, ok := aclObj.(*S3Acl)
		if !ok {
			return nil, fmt.Errorf("write to %s with ACL of unexpected type %T", p, aclObj)
		}
		return s3Acl.RequestACL, nil
	}
	return nil, nil
}

func (p *S3Path) WriteFile(ctx context.Context, data io.ReadSeeker, aclObj ACL) error {
	client, err := p.client(ctx)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Writing file %q", p)

	request := &s3.PutObjectInput{}
	request.Body = data
	request.Bucket = aws.String(p.bucket)
	request.Key = aws.String(p.key)

	var sseLog string
	request.ServerSideEncryption, sseLog, _ = p.getServerSideEncryption(ctx)

	request.ACL, err = p.getRequestACL(aclObj)
	if err != nil {
		return err
	}

	// We don't need Content-MD5: https://github.com/aws/aws-sdk-go/issues/208

	klog.V(8).Infof("Calling S3 PutObject Bucket=%q Key=%q SSE=%q ACL=%q", p.bucket, p.key, sseLog, aws.StringValue(request.ACL))

	_, err = client.PutObjectWithContext(ctx, request)
	if err != nil {
		if request.ACL != nil {
			return fmt.Errorf("error writing %s (with ACL=%q): %v", p, aws.StringValue(request.ACL), err)
		}
		return fmt.Errorf("error writing %s: %v", p, err)
	}

	return nil
}

// To prevent concurrent creates on the same file while maintaining atomicity of writes,
// we take a process-wide lock during the operation.
// Not a great approach, but fine for a single process (with low concurrency)
// TODO: should we enable versioning?
var createFileLockS3 sync.Mutex

func (p *S3Path) CreateFile(ctx context.Context, data io.ReadSeeker, acl ACL) error {
	createFileLockS3.Lock()
	defer createFileLockS3.Unlock()

	// Check if exists
	_, err := p.ReadFile(ctx)
	if err == nil {
		return os.ErrExist
	}

	if !os.IsNotExist(err) {
		return err
	}

	return p.WriteFile(ctx, data, acl)
}

// ReadFile implements Path::ReadFile
func (p *S3Path) ReadFile(ctx context.Context) ([]byte, error) {
	var b bytes.Buffer
	_, err := p.WriteTo(&b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteTo implements io.WriterTo
func (p *S3Path) WriteTo(out io.Writer) (int64, error) {
	ctx := context.TODO()
	client, err := p.client(ctx)
	if err != nil {
		return 0, err
	}

	klog.V(4).Infof("Reading file %q", p)

	request := &s3.GetObjectInput{}
	request.Bucket = aws.String(p.bucket)
	request.Key = aws.String(p.key)

	response, err := client.GetObjectWithContext(ctx, request)
	if err != nil {
		if AWSErrorCode(err) == "NoSuchKey" {
			return 0, os.ErrNotExist
		}
		return 0, fmt.Errorf("error fetching %s: %v", p, err)
	}
	defer response.Body.Close()

	n, err := io.Copy(out, response.Body)
	if err != nil {
		return n, fmt.Errorf("error reading %s: %v", p, err)
	}
	return n, nil
}

func (p *S3Path) ReadDir() ([]Path, error) {
	ctx := context.TODO()
	client, err := p.client(ctx)
	if err != nil {
		return nil, err
	}

	prefix := p.key
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	request := &s3.ListObjectsInput{}
	request.Bucket = aws.String(p.bucket)
	request.Prefix = aws.String(prefix)
	request.Delimiter = aws.String("/")

	klog.V(4).Infof("Listing objects in S3 bucket %q with prefix %q", p.bucket, prefix)
	var paths []Path
	err = client.ListObjectsPagesWithContext(ctx, request, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, o := range page.Contents {
			key := aws.StringValue(o.Key)
			if key == prefix {
				// We have reports (#548 and #520) of the directory being returned as a file
				// And this will indeed happen if the directory has been created as a file,
				// which seems to happen if you use some external tools to manipulate the S3 bucket.
				// We need to tolerate that, so skip the parent directory.
				klog.V(4).Infof("Skipping read of directory: %q", key)
				continue
			}
			child := &S3Path{
				s3Context: p.s3Context,
				bucket:    p.bucket,
				key:       key,
				etag:      o.ETag,
				scheme:    p.scheme,
				sse:       p.sse,
			}
			paths = append(paths, child)
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error listing %s: %v", p, err)
	}
	klog.V(8).Infof("Listed files in %v: %v", p, paths)
	return paths, nil
}

func (p *S3Path) ReadTree() ([]Path, error) {
	ctx := context.TODO()
	client, err := p.client(ctx)
	if err != nil {
		return nil, err
	}

	request := &s3.ListObjectsInput{}
	request.Bucket = aws.String(p.bucket)
	prefix := p.key
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	request.Prefix = aws.String(prefix)
	// No delimiter for recursive search

	var paths []Path
	err = client.ListObjectsPagesWithContext(ctx, request, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		for _, o := range page.Contents {
			key := aws.StringValue(o.Key)
			child := &S3Path{
				s3Context: p.s3Context,
				bucket:    p.bucket,
				key:       key,
				etag:      o.ETag,
				scheme:    p.scheme,
				sse:       p.sse,
			}
			paths = append(paths, child)
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error listing %s: %v", p, err)
	}
	return paths, nil
}

func (p *S3Path) getBucketDetails(ctx context.Context) (*S3BucketDetails, error) {
	bucketDetails, err := p.s3Context.getDetailsForBucket(ctx, p.bucket)
	if err != nil {
		return nil, err
	}

	return bucketDetails, nil
}

func (p *S3Path) client(ctx context.Context) (*s3.S3, error) {
	bucketDetails, err := p.getBucketDetails(ctx)
	if err != nil {
		return nil, err
	}

	client, err := p.s3Context.getClient(bucketDetails.region)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (p *S3Path) Base() string {
	return path.Base(p.key)
}

func (p *S3Path) PreferredHash() (*hashing.Hash, error) {
	return p.Hash(hashing.HashAlgorithmMD5)
}

func (p *S3Path) Hash(a hashing.HashAlgorithm) (*hashing.Hash, error) {
	if a != hashing.HashAlgorithmMD5 {
		return nil, nil
	}

	if p.etag == nil {
		return nil, nil
	}

	md5 := strings.Trim(*p.etag, "\"")

	md5Bytes, err := hex.DecodeString(md5)
	if err != nil {
		return nil, fmt.Errorf("Etag was not a valid MD5 sum: %q", *p.etag)
	}

	return &hashing.Hash{Algorithm: hashing.HashAlgorithmMD5, HashValue: md5Bytes}, nil
}

func (p *S3Path) GetHTTPsUrl(dualstack bool) (string, error) {
	ctx := context.TODO()

	bucketDetails, err := p.getBucketDetails(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get bucket details for %q: %w", p.String(), err)
	}

	var url string
	if dualstack {
		url = fmt.Sprintf("https://s3.dualstack.%s.amazonaws.com/%s/%s", bucketDetails.region, bucketDetails.name, p.Key())
	} else {
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucketDetails.name, bucketDetails.region, p.Key())
	}
	return strings.TrimSuffix(url, "/"), nil
}

func (p *S3Path) IsBucketPublic(ctx context.Context) (bool, error) {
	client, err := p.client(ctx)
	if err != nil {
		return false, err
	}

	result, err := client.GetBucketPolicyStatusWithContext(ctx, &s3.GetBucketPolicyStatusInput{
		Bucket: aws.String(p.bucket),
	})
	if err != nil && AWSErrorCode(err) != "NoSuchBucketPolicy" {
		return false, fmt.Errorf("from AWS S3 GetBucketPolicyStatusWithContext: %w", err)
	}
	if err == nil && aws.BoolValue(result.PolicyStatus.IsPublic) {
		return true, nil
	}
	return false, nil

	// We could check bucket ACLs also...

	// acl, err := client.GetBucketAclWithContext(ctx, &s3.GetBucketAclInput{
	// 	Bucket: &p.bucket,
	// })
	// if err != nil {
	// 	return false, fmt.Errorf("failed to get ACL for bucket %q: %w", p.bucket, err)
	// }

	// allowsAnonymousRead := false
	// for _, grant := range acl.Grants {
	// 	isAllUsers := false

	// 	switch aws.StringValue(grant.Grantee.URI) {
	// 	case "http://acs.amazonaws.com/groups/global/AllUsers":
	// 		isAllUsers = true
	// 	}

	// 	if isAllUsers {
	// 		permission := aws.StringValue(grant.Permission)
	// 		switch permission {
	// 		case "FULL_CONTROL":
	// 			klog.Warningf("bucket %q allows anonymous users full access", p.bucket)
	// 			allowsAnonymousRead = true
	// 		case "WRITE", "WRITE_ACP":
	// 			klog.Warningf("bucket %q allows anonymous users write access", p.bucket)
	// 			// it's not _read_ access
	// 		case "READ":
	// 			allowsAnonymousRead = true
	// 		case "READ_ACP":
	// 			// does not grant read
	// 		default:
	// 			klog.Warningf("bucket %q has unknown permission %q for anonymous access", p.bucket, permission)
	// 		}
	// 	}
	// }

	// return allowsAnonymousRead, nil
}

func (p *S3Path) IsPublic() (bool, error) {
	ctx := context.TODO()
	client, err := p.client(ctx)
	if err != nil {
		return false, err
	}
	acl, err := client.GetObjectAclWithContext(ctx, &s3.GetObjectAclInput{
		Bucket: &p.bucket,
		Key:    &p.key,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get grant for key %q in bucket %q: %w", p.key, p.bucket, err)
	}

	for _, grant := range acl.Grants {
		if aws.StringValue(grant.Grantee.URI) == "http://acs.amazonaws.com/groups/global/AllUsers" {
			return aws.StringValue(grant.Permission) == "READ", nil
		}
	}
	return false, nil
}

type terraformS3File struct {
	Bucket   string                   `json:"bucket" cty:"bucket"`
	Key      string                   `json:"key" cty:"key"`
	Content  *terraformWriter.Literal `json:"content,omitempty" cty:"content"`
	Acl      *string                  `json:"acl,omitempty" cty:"acl"`
	SSE      *string                  `json:"server_side_encryption,omitempty" cty:"server_side_encryption"`
	Provider *terraformWriter.Literal `json:"provider,omitempty" cty:"provider"`
}

type terraformDOFile struct {
	Bucket  string                   `json:"bucket" cty:"bucket"`
	Region  string                   `json:"region" cty:"region"`
	Key     string                   `json:"key" cty:"key"`
	Content *terraformWriter.Literal `json:"content,omitempty" cty:"content"`
}

func (p *S3Path) RenderTerraform(w *terraformWriter.TerraformWriter, name string, data io.Reader, acl ACL) error {
	ctx := context.TODO()

	bytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("reading data: %v", err)
	}

	// render DO's terraform
	if p.scheme == "do" {

		content, err := w.AddFileBytes("digitalocean_spaces_bucket_object", name, "content", bytes, false)
		if err != nil {
			return fmt.Errorf("rendering DO file: %v", err)
		}

		// retrieve space region from endpoint
		endpoint := os.Getenv("S3_ENDPOINT")
		if endpoint == "" {
			return fmt.Errorf("S3 Endpoint is empty")
		}
		region := strings.Split(endpoint, ".")[0]

		tf := &terraformDOFile{
			Bucket:  p.Bucket(),
			Region:  region,
			Key:     p.Key(),
			Content: content,
		}
		return w.RenderResource("digitalocean_spaces_bucket_object", name, tf)

	} else {
		bucketDetails, err := p.getBucketDetails(ctx)
		if err != nil {
			return err
		}

		tfProviderArguments := map[string]string{
			"region": bucketDetails.region,
		}
		w.EnsureTerraformProvider("aws", tfProviderArguments)

		content, err := w.AddFileBytes("aws_s3_object", name, "content", bytes, false)
		if err != nil {
			return fmt.Errorf("rendering S3 file: %v", err)
		}

		sse, _, err := p.getServerSideEncryption(ctx)
		if err != nil {
			return err
		}

		requestACL, err := p.getRequestACL(acl)
		if err != nil {
			return err
		}

		tf := &terraformS3File{
			Bucket:   p.Bucket(),
			Key:      p.Key(),
			Content:  content,
			SSE:      sse,
			Acl:      requestACL,
			Provider: terraformWriter.LiteralTokens("aws", "files"),
		}
		return w.RenderResource("aws_s3_object", name, tf)
	}

}

// AWSErrorCode returns the aws error code, if it is an awserr.Error, otherwise ""
func AWSErrorCode(err error) string {
	if awsError, ok := err.(awserr.Error); ok {
		return awsError.Code()
	}
	return ""
}
