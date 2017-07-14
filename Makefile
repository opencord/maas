SUBDIRS := automation config-generator harvester ip-allocator provisioner switchq
TARGETS := build publish clean test
PUBLIC_IMAGES := consul@sha256:0dc990ff3c44d5b5395475bcc5ebdae4fc8b67f69e17942a8b9793b3df74d290
I2 := consul:latest
I3 := consul

ANSIBLE_ARGS?=
MAKE_CONFIG?=config.mk

# [EXPERIMENTAL] Deployment via make is currently experimental
DEPLOY_INVENTORY=deploy-inv
DEPLOY_CONFIG=deploy-vars

# expands to lists of of the form:
# <target>_TARGETS := <subdir1>_<target> <subdir2>_<target>
$(foreach TARGET, $(TARGETS), $(eval $(TARGET)_TARGETS := $(addsuffix _$(TARGET), $(SUBDIRS))))

ifeq ($(realpath $(MAKE_CONFIG)),)
$(info Makefile configuration not found, defaults will be used.)
else
$(info Using makefile configuration "$(MAKE_CONFIG)")
endif

define recursive_rule
$1_$2:
	$$(MAKE) MAKE_CONFIG=$(realpath $(MAKE_CONFIG)) CONFIG=$(realpath $(CONFIG)) -C $1 $2
endef

$(foreach SUBDIR, $(SUBDIRS), $(foreach TARGET, $(TARGETS), $(eval $(call recursive_rule,$(SUBDIR),$(TARGET)))))

$(foreach TARGET, $(TARGETS), $(eval $(TARGET): $($(TARGET)_TARGETS)))

include help.mk

ifneq ($(realpath $(MAKE_CONFIG)),)
include $(MAKE_CONFIG)
endif

define public_image_rules
$2.image:
	docker pull $1
	@touch $$@

$2.publish: $2.image
	docker tag $1 $(DOCKER_REGISTRY)/$2:$(DEPLOY_DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$2:$(DEPLOY_DOCKER_TAG)
	@touch $$@

publish: $2.publish
endef

$(foreach PUBLIC_IMAGE, $(PUBLIC_IMAGES), $(eval $(call public_image_rules,$(PUBLIC_IMAGE),$(word 1,$(subst @, ,$(subst :, ,$(PUBLIC_IMAGE)))))))

prime:
	ansible-playbook -i $(DEPLOY_INVENTORY) --extra-vars=@$(DEPLOY_CONFIG) $(ANSIBLE_ARGS) prime-node.yml

deploy: publish
	ansible-playbook -i $(DEPLOY_INVENTORY) --extra-vars=@$(DEPLOY_CONFIG) $(ANSIBLE_ARGS) head-node.yml

config:
	@echo "hello"

clean:
