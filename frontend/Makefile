.PHONY: dev build run generate clean

# Generate templ files and run server
dev:
	templ generate && go run .

# Generate templ files
generate:
	templ generate

# Build binary
build:
	templ generate && go build -o bin/server .

# Run built binary
run:
	./bin/server

# Watch for changes (requires templ watch)
watch:
	templ generate --watch --proxy="http://localhost:3000" --cmd="go run ."

# Clean build artifacts
clean:
	rm -rf bin/
