help: ## Show this text.
	# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

all: ## Build binaries.

.PHONY: all test clean help

test: ## Run test.
	go test -v -race ./...
	go vet ./...

clean: ## Remove built files.
