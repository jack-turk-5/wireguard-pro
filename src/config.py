import logging
from os import environ
from itsdangerous import URLSafeTimedSerializer as Serializer

from utils import get_server_pubkey


class AppConfig:
    """A singleton class to hold application configuration."""
    def __init__(self):
        self.secret_key = None
        self.wg_endpoint = None
        self.wg_public_key = None
        self.wg_allowed_ips = None
        self.ts = None

    async def load(self):
        """Asynchronously load configuration from the environment and private key file."""
        self.secret_key = environ.get('SECRET_KEY')
        self.wg_endpoint = environ.get('WG_ENDPOINT')

        if not self.secret_key or not self.wg_endpoint:
            raise Exception('Missing SECRET_KEY and/or WG_ENDPOINT')

        self.ts = Serializer(self.secret_key, salt='auth-token')
        self.wg_public_key = await get_server_pubkey()
        self.wg_allowed_ips = environ.get('WG_ALLOWED_IPS', '0.0.0.0/0, ::/0')
        logging.info("Successfully loaded server public key and config.")


# Create a single, shared instance of the config
config = AppConfig()
