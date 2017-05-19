IMAGE_NAME=cord-provisioner
BINARY=entry-point
BUILD_TAG=build
PACKAGE_TAG=candidate

.PHONY: help
help:
	@echo "build     - create the binary"
	@echo "package   - package the binary into a docker container"
	@echo "clean     - remove tempory files and build artifacts"
	@echo "help      - this message"

BUILD_DATE=$(shell date -u +%Y-%m-%dT%TZ)
VCS_REF=$(shell git log --pretty=format:%H -n 1)
VCS_REF_DATE=$(shell git log --pretty=format:%cd --date=format:%FT%T%z -n 1)
BRANCHES=$(shell repo --color=never --no-pager branches 2>/dev/null | wc -l)
STATUS=$(shell repo --color=never --no-pager status . | tail -n +2 | wc -l)
MODIFIED=$(shell test $(BRANCHES) -eq 0 && test $(STATUS) -eq 0 || echo "[modified]")
BRANCH=$(shell repo --color=never --no-pager info -l -o | grep 'Manifest branch:' | awk '{print $$NF}')
VERSION=$(BRANCH)$(MODIFIED)

.PHONY: build
build:
	docker build -t $(IMAGE_NAME):$(BUILD_TAG) .

.PHONY: package
package:
	$(eval BUILD_ID := $(shell docker create $(IMAGE_NAME):$(BUILD_TAG)))
	$(eval BINDIR := $(shell mktemp -d))
	docker cp $(BUILD_ID):/service/$(BINARY) $(BINDIR)/$(BINARY)
	cp Dockerfile.release ssh-config ansible.cfg $(BINDIR)
	docker build -f $(BINDIR)/Dockerfile.release -t $(IMAGE_NAME):$(PACKAGE_TAG) --label org.label-schema.build-date=$(BUILD_DATE) --label org.label-schema.vcs-ref=$(VCS_REF) --label org.label-schema.vcs-ref-date=$(VCS_REF_DATE) --label org.label-schema.version=$(VERSION) $(BINDIR)
	docker rm -f $(BUILD_ID)
	rm -r $(BINDIR)

.PHONY: clean
clean:
	@echo ""
