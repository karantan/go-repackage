.PHONY: build clean deploy

build:
	env GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o bootstrap repackage/main.go

clean:
	rm -rf ./bin bootstrap

deploy: clean build
	sls deploy --verbose --aws-profile <PROFILE>
