IMAGE_NAME=localhost/wireguard/wireguard-pro
CONTAINER_NAME=wireguard-pro
WG_KEYDATA=wg-keydata
UI_DATA=wg-pro-data

.PHONY: build reload start stop credentials clean upgrade deploy status logs test

## Upgrade container (build + reload)
upgrade: build reload status

## Deploy: secrets + build + start
deploy: credentials build start status

## Build the container and reload systemd
build:
	podman build -t $(IMAGE_NAME):latest -f Containerfile
	systemctl --user daemon-reload

## Run the test suite in an isolated container (only Podman required on the host).
## --device passes /dev/net/tun through so TestEndToEnd exercises the real
## path instead of skipping; it still skips cleanly on hosts without it.
test:
	podman build -t wireguard-pro-test -f hack/Containerfile.test .
	podman run --rm --device /dev/net/tun wireguard-pro-test

## Reload the container and socket (zero downtime)
reload:
	-systemctl --user stop $(CONTAINER_NAME).socket $(CONTAINER_NAME).service
	systemctl --user daemon-reload
	systemctl --user restart $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Create UI credentials if missing
credentials:
	@echo "→ Creating admin-user/admin-pass secrets"
	@./secrets/create_credentials.py

## Start container and socket
start:
	systemctl --user enable --now $(CONTAINER_NAME).socket
	systemctl --user start $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Stop container and socket
stop:
	-systemctl --user disable --now $(CONTAINER_NAME).socket
	systemctl --user stop $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Clean container and image
clean:
	-podman container rm -f $(CONTAINER_NAME)
	-podman rmi $(IMAGE_NAME):latest
	-podman volume rm $(WG_KEYDATA) $(UI_DATA)
	systemctl --user daemon-reload

## Check container status
status:
	systemctl --user status $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Follow logs
logs:
	journalctl --user-unit $(CONTAINER_NAME).service -f