IMAGE_NAME=localhost/wireguard/wireguard-pro
CONTAINER_NAME=wireguard-pro
SECRETS_NAME=wg-privatekey
SECRETS_NAME_PUB=wg-publickey

.PHONY: build optimize reload start stop secrets clean upgrade deploy status logs

upgrade-optimal: optimize reload status

## Upgrade container (build + reload)
upgrade: build reload status

deploy-optimal: secrets optimize start status

## Deploy: secrets + build + start
deploy: secrets build start status

## Build the container and reload systemd
build:
	podman build -t $(IMAGE_NAME):latest -f Containerfile
	systemctl --user daemon-reload

## Build optimized container (strip binaries first)
optimize:
	podman build --build-arg STRIP=true -t $(IMAGE_NAME):latest -f Containerfile
	systemctl --user daemon-reload

## Reload the container and socket (zero downtime)
reload:
	-systemctl --user stop $(CONTAINER_NAME).service $(CONTAINER_NAME).socket
	systemctl --user restart $(CONTAINER_NAME).socket
	systemctl --user restart $(CONTAINER_NAME).service

## Create secrets if missing
secrets:
	@echo "Checking/creating Podman secret..."
	podman secret exists $(SECRETS_NAME) || \
	wg genkey | tee secrets/$(SECRETS_NAME) | wg pubkey > secrets/$(SECRETS_NAME_PUB)
	podman secret create $(SECRETS_NAME) ./secrets/$(SECRETS_NAME)

## Start container and socket
start:
	systemctl --user start $(CONTAINER_NAME).service
	systemctl --user start $(CONTAINER_NAME).socket

## Stop container and socket
stop:
	systemctl --user stop $(CONTAINER_NAME).socket
	-systemctl --user stop $(CONTAINER_NAME).service

## Clean container and image
clean:
	-podman container rm -f $(CONTAINER_NAME)
	-podman rmi $(IMAGE_NAME):latest
	systemctl --user daemon-reload

## Check container status
status:
	systemctl --user status $(CONTAINER_NAME).service
	systemctl --user status $(CONTAINER_NAME).socket

## Follow logs
logs:
	journalctl --user-unit $(CONTAINER_NAME).service -f