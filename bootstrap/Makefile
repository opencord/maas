help:
	-@echo "Available actions"
	-@echo "    docker      - builds the docker container"

docker: bootstrap.py
	docker build -t cord/maas-bootstrap:0.1-prerelease .

run:
	docker run -ti --rm=true cord/maas-bootstrap:0.1-prerelease --apikey=$(CORD_APIKEY) --sshkey="$(CORD_SSHKEY)" --url=$(CORD_URL)
