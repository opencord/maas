.DEFAULT_GOAL := help

help:
	@echo "Available targets:"
	@echo "    build     - builds any artifacts"
	@echo "    publish   - publishes any built artifacts to a deployment server"
	@echo "    clean     - remove tempory files and build artifacts"
	@echo "    test      - executes any unit tests on the project"
	@echo "    help      - this message"
	@echo ""
	@echo "Available environment variables:"
	@echo "    PROJECT_PREFIX - defines a prefix to prepend to all Docker image names"
	@echo "    PACKAGE_TAG    - defines the TAG to use on the public Docker image that is the build artifact"
	@echo "    REGISTRY       - name of the registry to which to publish Docker images"
	@echo "    DOCKER_ARGS    - additional arguments to pass to the Docker build command"
