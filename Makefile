.PHONY: run test templ tailwind generate build tidy

# Local tools
TEMPL := go run github.com/a-h/templ/cmd/templ
TAILWIND := ./bin/tailwindcss

# Regenerate templ Go code and Tailwind CSS, then run the server.
run: generate
	go run ./cmd/server

# Compile templ templates to Go.
templ:
	$(TEMPL) generate

# Build the Tailwind stylesheet from the templ output. Requires bin/tailwindcss
# (the standalone CLI — download once, see README). No Node.
tailwind:
	$(TAILWIND) -i web/styles/input.css -o web/static/app.css --minify

# templ first (so Tailwind can scan the generated classes), then CSS.
generate: templ tailwind

build: generate
	go build ./...

test:
	go test ./...

tidy:
	go mod tidy
