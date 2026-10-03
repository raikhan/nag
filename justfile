# Build the app
build:
    go build -o dist/nag .

# Run a fresh build
run: build
    ./dist/nag

# Install the app
install: build
    mv dist/nag ~/go/bin

