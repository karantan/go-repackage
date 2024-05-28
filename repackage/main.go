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
	"repackage/storage"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/klauspost/compress/zstd"
)

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
			return nil, err
		}

		if header.Typeflag == tar.TypeDir {
			continue
		}

		fileInZip, err := zipWriter.Create(header.Name)
		if err != nil {
			return nil, err
		}

		if _, err := io.Copy(fileInZip, tarReader); err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return zipBuf.Bytes(), nil
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var req Request
	err := json.Unmarshal([]byte(request.Body), &req)
	if err != nil {
		return Response{StatusCode: 400, Body: "Invalid request payload"}, nil
	}

	reader, originalFilename, err := downloadFile(req.URL)
	if err != nil {
		return Response{StatusCode: 500, Body: "Failed to download file"}, nil
	}

	zipBytes, err := repackageFile(reader)
	if err != nil {
		return Response{StatusCode: 500, Body: "Failed to repackage file"}, nil
	}

	zipFilename := originalFilename + ".zip"
	objectKey := "converted/" + zipFilename

	bucket := storage.NewB2Client(os.Getenv("BUCKET_NAME"))
	if err := storage.Write(bucket.S3Client, bucket.BucketName, objectKey, bytes.NewReader(zipBytes)); err != nil {
		return Response{StatusCode: 500, Body: "Failed to repackage file"}, nil
	}

	presignedReq, err := storage.GetPresignedObject(bucket.Presigner, bucket.BucketName, objectKey, int64(3600))
	if err != nil {
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
