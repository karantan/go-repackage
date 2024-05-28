package storage

// Package storage provides functionalities to interact with
// Blackblaze B2 Storage using the AWS SDK. This package provides
// utility functions to upload files.
//
// Usage of this package requires specific environment variables to be set:
//   - B2_KEY_ID: Represents the Blackblaze Key ID.
//   - B2_APPLICATION_KEY: Represents the access key Application Key.
//
// For more details on Blackblaze B2 Storage with AWS SDK, refer to:
// https://www.backblaze.com/docs/cloud-storage-use-the-aws-sdk-for-go-with-backblaze-b2
// https://github.com/awsdocs/aws-doc-sdk-examples/blob/main/gov2/s3

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// Bucket encapsulates the Amazon Simple Storage Service (Amazon S3) actions.
// It contains S3Client, an Amazon S3 service client that is used to perform bucket
// and object actions.
type Bucket struct {
	S3Client   S3Client
	Presigner  Presigner
	BucketName string
}

// NewB2Client creates and returns a new Bucket instance configured for
// Blackblaze B2 storage. It fetches necessary credentials and account ID
// from environment variables and sets up the custom endpoint resolver
// for Blackblaze's B2 storage.
func NewB2Client(bucketName string) *Bucket {
	keyID := os.Getenv("B2_KEY_ID")
	applicationKey := os.Getenv("B2_APPLICATION_KEY")
	endpoint := "https://s3.us-west-000.backblazeb2.com"

	if keyID == "" {
		log.Fatalf("Missing B2_KEY_ID env. var.")
	}
	if applicationKey == "" {
		log.Fatalf("Missing B2_APPLICATION_KEY env. var.")
	}

	B2Resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint}, nil
		})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(B2Resolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(keyID, applicationKey, ""),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	s3client := s3.NewFromConfig(cfg)
	return &Bucket{
		S3Client:   s3client,
		Presigner:  s3.NewPresignClient(s3client),
		BucketName: bucketName}

}

// Write uploads a local file to Blackblaze B2 Storage.
//
// Usage:
//
//	err := storage.Write(s3Client, "path/in/bucket", "/local/path/to/file")
//	if err != nil {
//	    log.Fatalf("Upload error: %v", err)
//	}

func Write(api S3Client, bucket_name, objectKey string, reader *bytes.Reader) error {
	putObject := &s3.PutObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(objectKey),
		Body:   reader,
	}
	_, err := api.PutObject(context.TODO(), putObject)
	return err
}

// PresigneClient encapsulates the Amazon Simple Storage Service (Amazon S3) presign actions
// used in the examples.
// It contains PresignClient, a client that is used to presign requests to Amazon S3.
// Presigned requests contain temporary credentials and can be made from any HTTP client.
// type PresigneClient struct {
// 	PresignClient *s3.PresignClient
// }

type Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// GetPresignedObject makes a presigned request that can be used to get an object from a bucket.
// The presigned request is valid for the specified number of seconds.
func GetPresignedObject(
	p Presigner, bucket_name, objectKey string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error) {

	ctx := context.TODO()
	bucketObject := &s3.GetObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(objectKey),
	}
	options := func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(lifetimeSecs * int64(time.Second))
	}
	request, err := p.PresignGetObject(ctx, bucketObject, options)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get a presigned request to get %v:%v. Here's why: %v\n",
			bucket_name, objectKey, err)
	}
	return request, err
}
