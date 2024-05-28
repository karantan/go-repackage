package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
)

// Mock server for testing
func startMockServer(_ *testing.T, tarZstData []byte) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarZstData)
	})
	server := httptest.NewServer(handler)
	return server
}

// Helper function to create a .tar.zst file for testing
func createTarZstFile(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zstdWriter, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("Failed to create zstd writer: %v", err)
	}

	tarWriter := tar.NewWriter(zstdWriter)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}
	tarWriter.Close()
	zstdWriter.Close()

	return buf.Bytes()
}

func TestDownloadFile(t *testing.T) {
	files := map[string]string{
		"file1.tar.zst": "This is a test file.",
	}
	tarZstData := createTarZstFile(t, files)
	server := startMockServer(t, tarZstData)
	defer server.Close()

	reader, filename, err := downloadFile(server.URL + "/file1.tar.zst")
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.Equal(t, "file1", filename)
	reader.Close()
}

func TestRepackageFile(t *testing.T) {
	files := map[string]string{
		"file1.tar.zst": "This is a test file.",
	}
	tarZstData := createTarZstFile(t, files)

	reader := io.NopCloser(bytes.NewReader(tarZstData))
	zipBytes, err := repackageFile(reader)
	assert.NoError(t, err)

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	assert.NoError(t, err)

	assert.Len(t, zipReader.File, 1)
	assert.Equal(t, "file1.tar.zst", zipReader.File[0].Name)

	rc, err := zipReader.File[0].Open()
	assert.NoError(t, err)
	defer rc.Close()

	content, err := io.ReadAll(rc)
	assert.NoError(t, err)
	assert.Equal(t, "This is a test file.", string(content))
}

func TestHandler(t *testing.T) {
	files := map[string]string{
		"file1.tar.zst": "This is a test file.",
	}
	tarZstData := createTarZstFile(t, files)
	server := startMockServer(t, tarZstData)
	defer server.Close()

	requestBody, _ := json.Marshal(Request{
		URL: server.URL,
	})
	request := events.APIGatewayProxyRequest{
		Body: string(requestBody),
	}

	response, err := Handler(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)
	assert.True(t, response.IsBase64Encoded)

	zipBytes, err := base64.StdEncoding.DecodeString(response.Body)
	assert.NoError(t, err)

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	assert.NoError(t, err)

	assert.Len(t, zipReader.File, 1)
	assert.Equal(t, "file1.tar.zst", zipReader.File[0].Name)

	rc, err := zipReader.File[0].Open()
	assert.NoError(t, err)
	defer rc.Close()

	content, err := io.ReadAll(rc)
	assert.NoError(t, err)
	assert.Equal(t, "This is a test file.", string(content))
}
