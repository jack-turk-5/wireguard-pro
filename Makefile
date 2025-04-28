IMAGE_NAME=localhost/wireguard/wireguard-pro
CONTAINER_NAME=wireguard-pro
SECRETS_NAME=wg-pro-privatekey

.PHONY: build reload start stop secrets clean upgrade deploy status logs

## Build the container and reload systemd
build:
	podman build -t $(IMAGE_NAME):latest -f Containerfile
	systemctl --user daemon-reload

## Reload the container and socket (zero downtime)
reload:
	podman container stop --time 10 $(CONTAINER_NAME) || true
	systemctl --user restart $(CONTAINER_NAME).service
	systemctl --user restart $(CONTAINER_NAME).socket

## Create secrets if missing
secrets:
	@echo "Checking/creating Podman secret..."
	podman secret exists $(SECRETS_NAME) || podman secret create $(SECRETS_NAME) ./secrets/wg_privatekey

## Start container and socket
start:
	systemctl --user start $(CONTAINER_NAME).service
	systemctl --user start $(CONTAINER_NAME).socket

## Stop container and socket
stop:
	systemctl --user stop $(CONTAINER_NAME).service
	systemctl --user stop $(CONTAINER_NAME).socket

## Clean container and image
clean:
	podman container rm -f $(CONTAINER_NAME) || true
	podman rmi $(IMAGE_NAME):latest || true
	systemctl --user daemon-reload

## Upgrade container (build + reload)
upgrade: build reload

## Deploy: secrets + build + start
deploy: secrets build start

## Check container status
status:
	systemctl --user status $(CONTAINER_NAME).service
	systemctl --user status $(CONTAINER_NAME).socket

## Follow logs
logs:
	journalctl --user-unit $(CONTAINER_NAME).service -f