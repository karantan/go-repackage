package storage

import (
	"bytes"
	"context"
	mock_storage "repackage/mocks"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestWrite(t *testing.T) {
	expectedBody := []byte("this is the body")
	ctrl := gomock.NewController(t)
	m := mock_storage.NewMockS3Client(ctrl)

	ctx := context.TODO()
	bucket := "bucket"
	key := "key/test.pdf"

	fakeInput := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(expectedBody),
	}

	fakeOutput := &s3.PutObjectOutput{}

	m.EXPECT().PutObject(ctx, gomock.AssignableToTypeOf(fakeInput)).Return(fakeOutput, nil)
	assert := assert.New(t)
	err := Write(m, bucket, key, bytes.NewReader(expectedBody))
	assert.NoError(err)
}

func TestGetPresignedObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock_storage.NewMockPresigner(ctrl)

	bucket := "bucket"
	key := "key"
	lifetimeSec := int64(100)

	ctx := context.TODO()
	fakeBucket := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	fakeRequest := &v4.PresignedHTTPRequest{
		URL: "http://foo.com",
	}
	// optFns is a variadic parameter and it can't be easily mocked. use gomock.Any
	m.EXPECT().PresignGetObject(ctx, fakeBucket, gomock.Any()).Return(fakeRequest, nil)
	assert := assert.New(t)
	got, err := GetPresignedObject(m, bucket, key, lifetimeSec)
	assert.NoError(err)
	assert.Equal("http://foo.com", got.URL)
}
