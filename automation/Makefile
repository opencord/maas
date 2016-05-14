.PHONY: help
help:
	@echo "image     - create docker image for the MAAS deploy flow utility"
	@echo "save      - save the docker image for the MAAS deployment flow utility to a local tar file"
	@echo "clean     - remove any generated files"
	@echo "help      - this message"

.PHONY: image
image:
	docker build -t cord/maas-automation:0.1-prerelease .

save: image
	docker save -o cord_maas-automation.1-prerelease.tar cord/maas-automation:0.1-prerelease

.PHONT: clean
clean:
	rm -f cord_maas-automation.1-prerelease.tar
