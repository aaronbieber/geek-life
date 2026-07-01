build:
    env GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o builds/geek-life_darwin-arm64 ./app
