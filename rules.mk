.PRECIOUS: Dockerfile.image

ifneq ($(MAKE_CONFIG),)
include $(MAKE_CONFIG)
endif

PROJECT_PREFIX?=opencord/maas-

ifeq ($(DOCKER_TAG),)
DOCKER_TAG:=candidate
endif

BUILD_DATE=$(shell date -u +%Y-%m-%dT%TZ)
VCS_REF=$(shell git log --pretty=format:%H -n 1)
VCS_REF_DATE=$(shell git log --pretty=format:%cd --date=format:%FT%T%z -n 1)
BRANCHES=$(shell repo --color=never --no-pager branches 2>/dev/null | wc -l)
STATUS=$(shell repo --color=never --no-pager status . | tail -n +2 | wc -l)
MODIFIED=$(shell test $(BRANCHES) -eq 0 && test $(STATUS) -eq 0 || echo "[modified]")
BRANCH=$(shell repo --color=never --no-pager info -l -o | grep 'Manifest branch:' | awk '{print $$NF}')
VERSION=$(BRANCH)$(MODIFIED)

include ../help.mk

build: $(addsuffix .image,$(IMAGES))

publish: $(addsuffix .publish,$(IMAGES))

test:
	@echo "Really should have some tests"

%.image : Dockerfile.%
	docker build $(DOCKER_ARGS) -f Dockerfile.$(basename $@) \
		-t $(PROJECT_PREFIX)$(basename $@):$(DOCKER_TAG) \
		--label org.label-schema.build-date=$(BUILD_DATE) \
		--label org.label-schema.vcs-ref=$(VCS_REF) \
		--label org.label-schema.vcs-ref-date=$(VCS_REF_DATE) \
		--label org.label-schema.version=$(VERSION) .

%.publish :
ifdef DOCKER_REGISTRY
	$(eval BASENAME := $(basename $@):$(DOCKER_TAG))
	docker tag $(PROJECT_PREFIX)$(BASENAME) $(DOCKER_REGISTRY)/$(PROJECT_PREFIX)$(BASENAME)
	docker push $(DOCKER_REGISTRY)/$(PROJECT_PREFIX)$(BASENAME)
else
	@echo "No registry was specified, cannot PUSH image"
endif

clean:
