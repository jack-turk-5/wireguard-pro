import logging
import asyncio
from os import environ
from itsdangerous import URLSafeTimedSerializer as Serializer

from utils import get_server_pubkey, _run_command


class AppConfig:
    """A singleton class to hold application configuration."""

    def __init__(self):
        self.secret_key = None
        self.wg_endpoint = None
        self.wg_public_key = None
        self.wg_allowed_ips = None
        self.wg_dns_server = None
        self.ts = None
        self.wg_ipv4_base_addr = None
        self.wg_ipv6_base_addr = None

        self._is_loaded = False
        self._load_lock = asyncio.Lock()

    async def load(self):
        """
        Asynchronously load configuration. This method is now idempotent and safe
        to call multiple times.
        """
        async with self._load_lock:
            if self._is_loaded:
                return

            self.secret_key = environ.get("SECRET_KEY")
            self.wg_endpoint = environ.get("WG_ENDPOINT")

            if not self.secret_key or not self.wg_endpoint:
                raise Exception("Missing SECRET_KEY and/or WG_ENDPOINT")

            self.ts = Serializer(self.secret_key, salt="auth-token")
            self.wg_public_key = await get_server_pubkey()
            self.wg_allowed_ips = environ.get("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0")
            self.wg_dns_server = environ.get("WG_DNS_SERVER", "1.1.1.1")
            self.wg_ipv4_base_addr = environ.get("WG_IPV4_BASE_ADDR", "10.8.0.1")
            self.wg_ipv6_base_addr = environ.get(
                "WG_IPV6_BASE_ADDR", "fd86:ea04:1111::1"
            )

            self._is_loaded = True
            logging.info("Successfully loaded server public key and config.")

    async def get_server_pubkey(self):
        """
        Derive server public key from stored private key asynchronously.
        """
        async with open("/etc/wireguard/privatekey") as f:
            priv = (await f.read()).strip()
        return await _run_command("wg pubkey", stdin_input=priv)


# Create a single, shared instance of the config
config = AppConfig()


async def get_config() -> AppConfig:
    """
    Returns the globally available, loaded configuration object.
    This is the single entry point for accessing configuration.
    """
    if not config._is_loaded:
        await config.load()
    return config
