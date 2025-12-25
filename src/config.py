import logging
from os import environ
from itsdangerous import URLSafeTimedSerializer as Serializer
from cli import run_command
from asyncio import Lock
from aiofiles import open


class AppConfig:
    """A singleton class to hold application configuration."""

    def __init__(self):
        self.secret_key = None
        self.wg_endpoint = None
        self.wg_port = None
        self.wg_public_key = None
        self.wg_allowed_ips = None
        self.wg_dns_server = None
        self.ts = None
        self.wg_ipv4_base_addr = None
        self.wg_ipv6_base_addr = None

        self._is_loaded = False
        self._load_lock = Lock()

    async def _get_server_pubkey(self):
        """
        Derive server public key from stored private key asynchronously.
        """
        async with open("/etc/wireguard/privatekey") as f:
            priv = (await f.read()).strip()
        return await run_command("wg pubkey", stdin_input=priv)

    async def load(self):
        """
        Asynchronously load configuration. This method is now idempotent and safe
        to call multiple times.
        """
        async with self._load_lock:
            if self._is_loaded:
                return

            self.secret_key = environ.get("SECRET_KEY")
            self.wg_endpoint = environ.get("WG_ENDPOINT", "").strip(" '\"")
            self.wg_port = environ.get("WG_PORT", "").strip(" '\"")

            if not self.secret_key or not self.wg_endpoint or not self.wg_port:
                raise Exception("Missing SECRET_KEY, WG_PORT, or WG_ENDPOINT")

            self.ts = Serializer(self.secret_key, salt="auth-token")
            self.wg_public_key = await self._get_server_pubkey()
            self.wg_allowed_ips = environ.get("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0")
            self.wg_dns_server = environ.get("WG_DNS_SERVER", "1.1.1.1")
            self.wg_ipv4_base_addr = environ.get("WG_IPV4_BASE_ADDR", "10.8.0.1")
            self.wg_ipv6_base_addr = environ.get(
                "WG_IPV6_BASE_ADDR", "fd86:ea04:1111::1"
            )
            self._is_loaded = True
            logging.info("Successfully loaded server public key and config.")


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
