import logging
from aiofiles import open as aio_open
from itsdangerous import URLSafeTimedSerializer as Serializer
from pydantic_settings import BaseSettings, SettingsConfigDict

from cli import run_command


class AppConfig(BaseSettings):
    """Application configuration loaded from environment variables using Pydantic Settings."""

    secret_key: str
    wg_host: str
    wg_port: str
    wg_allowed_ips: str = "0.0.0.0/0, ::/0"
    wg_dns_server: str = "1.1.1.1"
    wg_ipv4_base_addr: str = "10.8.0.1"
    wg_ipv6_base_addr: str = "fd86:ea04:1111::1"
    db_file: str = "/data/peers.db"

    # Derived attributes populated asynchronously after initialization
    wg_public_key: str | None = None
    ts: Serializer | None = None

    _instance = None

    model_config = SettingsConfigDict(
        arbitrary_types_allowed=True,
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    def __init__(self, **kwargs):
        """
        Override default ctor to make type checker happy
        """
        super.__init__(**kwargs)

    @classmethod
    def get(cls):
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance


async def _get_server_pubkey() -> str:
    """
    Derive server public key from stored private key asynchronously.
    """
    async with aio_open("/etc/wireguard/privatekey") as f:
        priv = (await f.read()).strip()
    return await run_command("wg pubkey", stdin_input=priv)


async def get_config() -> AppConfig:
    """
    Returns the globally available, loaded configuration object.
    This is the single entry point for accessing configuration.
    """
    settings = AppConfig.get()
    if settings.wg_public_key is None:
        settings.ts = Serializer(settings.secret_key, salt="auth-token")
        settings.wg_public_key = await _get_server_pubkey()
        logging.info("Successfully loaded server public key and config.")
    return settings
