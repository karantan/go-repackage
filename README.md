# go-repackage

TarGZ to Zip Converter Lambda Function.

This repository contains a Go AWS Lambda function designed to receive a URL link to a .tar.gz file, unpack it, compress the contents into a .zip file, and return the resulting .zip file.

## Usage

Send a POST request to the API Gateway endpoint with the following JSON payload:

```json
{
    "url": "https://example.com/file.tar.gz"
}
