IMAGE_NAME=localhost/wireguard/wireguard-pro
CONTAINER_NAME=wireguard-pro
BORINGTUN_DATA=boringtun-data
UI_DATA=wg-pro-data
PRIV_KEY=wg-privatekey
PUB_KEY=wg-publickey

.PHONY: build optimize reload start stop wg-keys credentials clean upgrade deploy status logs

upgrade-optimal: optimize reload status

## Upgrade container (build + reload)
upgrade: build reload status

deploy-optimal: wg-keys credentials optimize start status

## Deploy: secrets + build + start
deploy: wg-keys credentials build start status

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
	-systemctl --user stop $(CONTAINER_NAME).socket $(CONTAINER_NAME).service
	systemctl --user daemon-reload
	systemctl --user restart $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Create wg-keys if missing (tempfile fallback)
wg-keys:
	@echo "→ ensuring keys exist"
	@if podman secret exists "$(PRIV_KEY)" >/dev/null 2>&1; then \
	  echo "✓ Secret '$(PRIV_KEY)' already registered, skipping."; \
	else \
	  echo "✗ Not found, generating keys (tempfile)"; \
	  umask 077; \
	  tmp=$$(mktemp); \
	  wg genkey > $$tmp; \
	  podman secret create "$(PRIV_KEY)" $$tmp; \
	  wg pubkey < $$tmp | podman secret create "$(PUB_KEY)" -; \
	  rm -f $$tmp; \
	  echo "✓ Keys created and stored in Podman secrets, no files left on disk!"; \
	fi

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
	-podman volume rm $(BORINGTUN_DATA) $(UI_DATA)
	systemctl --user daemon-reload

## Check container status
status:
	systemctl --user status $(CONTAINER_NAME).socket $(CONTAINER_NAME).service

## Follow logs
logs:
	journalctl --user-unit $(CONTAINER_NAME).service -f