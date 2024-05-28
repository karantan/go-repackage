package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"repackage/logger"
	"repackage/storage"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/klauspost/compress/zstd"
)

var log = logger.New("main", false)

// Request is the struct for the incoming request payload
type Request struct {
	URL string `json:"url"`
}

// Response is the struct for the outgoing response
type Response events.APIGatewayProxyResponse

// downloadFile downloads the file from the provided URL and returns a ReadCloser
func downloadFile(fileURL string) (io.ReadCloser, string, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, "", err
	}

	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return nil, "", err
	}

	originalFilename := path.Base(parsedURL.Path)
	if strings.HasSuffix(originalFilename, ".tar.zst") {
		originalFilename = strings.TrimSuffix(originalFilename, ".tar.zst")
	}

	return resp.Body, originalFilename, nil
}

// repackageFile reads from the tar.zst stream, decompresses it, and writes the contents to a zip buffer
func repackageFile(reader io.ReadCloser) ([]byte, error) {
	defer reader.Close()

	zstdReader, err := zstd.NewReader(reader)
	if err != nil {
		log.Errorf("Failed to create new zstd reader: %v", err)
		return nil, err
	}
	defer zstdReader.Close()

	tarReader := tar.NewReader(zstdReader)
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("Failed to advance to the next entry in the tar archive: %v", err)
			return nil, err
		}

		if header.Typeflag == tar.TypeDir {
			continue
		}

		fileInZip, err := zipWriter.Create(header.Name)
		if err != nil {
			log.Errorf("Failed to create new zip writer: %v", err)
			return nil, err
		}

		if _, err := io.Copy(fileInZip, tarReader); err != nil {
			log.Errorf("Failed to copy tar content to zip archive: %v", err)
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		log.Errorf("Failed to close zip writer: %v", err)
		return nil, err
	}

	return zipBuf.Bytes(), nil
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var req Request
	err := json.Unmarshal([]byte(request.Body), &req)
	if err != nil {
		log.Errorf("Invalid request payload: %v", err)
		return Response{StatusCode: 400, Body: "Invalid request payload"}, nil
	}

	reader, originalFilename, err := downloadFile(req.URL)
	if err != nil {
		log.Errorf("Failed to download file: %v", err)
		return Response{StatusCode: 500, Body: "Failed to download file"}, nil
	}

	zipBytes, err := repackageFile(reader)
	if err != nil {
		log.Errorf("Failed to repackage file: %v", err)
		return Response{StatusCode: 500, Body: "Failed to repackage file"}, nil
	}

	timestamp := time.Now().Format("2006-01-02 15:04")
	zipFilename := fmt.Sprintf("%s-%s.zip", originalFilename, timestamp)

	bucket := storage.NewB2Client(os.Getenv("BUCKET_NAME"))
	if err := storage.Write(bucket.S3Client, bucket.BucketName, zipFilename, bytes.NewReader(zipBytes)); err != nil {
		log.Errorf("Failed to upload file: %v", err)
		return Response{StatusCode: 500, Body: "Failed to upload file"}, nil
	}

	presignedReq, err := storage.GetPresignedObject(bucket.Presigner, bucket.BucketName, zipFilename, int64(3600))
	if err != nil {
		log.Errorf("Failed to generate presigned URL: %v", err)
		return Response{StatusCode: 500, Body: "Failed to generate presigned URL"}, nil
	}

	return Response{
		StatusCode: 200,
		Body:       fmt.Sprintf(`{"url":"%s"}`, presignedReq.URL),
	}, nil
}

func main() {
	lambda.Start(Handler)
}
