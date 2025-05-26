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
	systemctl daemon-reload

## Build optimized container (strip binaries first)
optimize:
	podman build --build-arg STRIP=true -t $(IMAGE_NAME):latest -f Containerfile
	systemctl daemon-reload

## Reload the container and socket (zero downtime)
reload:
	-systemctl stop $(CONTAINER_NAME).socket $(CONTAINER_NAME).service
	systemctl daemon-reload
	systemctl restart $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Create secrets if missing
secrets:
	@echo "→ ensuring secrets/ directory exists…"
	mkdir -p secrets
	@echo "→ checking if Podman secret '$(SECRETS_NAME)' exists…"
	@if podman secret exists $(SECRETS_NAME) >/dev/null 2>&1; then \
	  echo "✓ secret '$(SECRETS_NAME)' already registered, skipping."; \
	else \
	  echo "✗ not found, generating WireGuard keypair…"; \
	  wg genkey | tee secrets/$(SECRETS_NAME) | wg pubkey > secrets/$(SECRETS_NAME_PUB); \
	  echo "✎ creating Podman secret '$(SECRETS_NAME)' from private key…"; \
	  podman secret create $(SECRETS_NAME) secrets/$(SECRETS_NAME); \
	  echo "✓ secret registered."; \
	fi

## Start container and socket
start:
	systemctl enable --now $(CONTAINER_NAME).socket
	systemctl start $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Stop container and socket
stop:
	-systemctl disable --now $(CONTAINER_NAME).socket
	systemctl stop $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Clean container and image
clean:
	-podman container rm -f $(CONTAINER_NAME)
	-podman rmi $(IMAGE_NAME):latest
	systemctl daemon-reload

## Check container status
status:
	systemctl status $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Follow logs
logs:
	journalctl $(CONTAINER_NAME).service -f