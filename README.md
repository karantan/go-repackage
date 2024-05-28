# go-repackage

TAR.ZST to ZIP Converter Lambda Function.

This repository contains a Go AWS Lambda function designed to receive a URL link to a `.tar.zst` file, unpack it, compress the contents into a `.zip` file, upload the resulting `.zip` file to an S3 bucket, and return the URL of the uploaded `.zip` file.

## Usage

Send a POST request to the API Gateway endpoint with the following JSON payload:

```json
{
    "url": "https://example.com/file.tar.zst"
}
