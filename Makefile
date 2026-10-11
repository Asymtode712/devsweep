VERSION ?= 0.1.0
LDFLAGS := -s -w -X github.com/Asymtode712/devsweep/internal/cli.Version=$(VERSION)
TARGETS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: build test dist site clean

build:
	go build -ldflags "$(LDFLAGS)" -o devsweep .

test:
	go vet ./...
	go test ./...

dist:
	@mkdir -p dist
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/devsweep-$$os-$$arch$$ext . || exit 1; \
	done

site: build
	./devsweep rules --json > site/src/data/rules.json
	cd site && npm ci && npm run build
	@echo "site built in site/dist — preview with: cd site && npm run preview"

clean:
	rm -rf devsweep dist
