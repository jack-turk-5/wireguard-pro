import logging
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from config import config
from db import init_db, add_or_update_user_db
from scheduler import scheduler
from auth import router as auth_router
from api import router as api_router

# --- Configuration & Logging ---
logging.basicConfig(level=logging.INFO)

# --- Lifespan Management (for startup/shutdown events) ---
@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    try:
        init_db()
        app.state.config = await config.load()

        # Seed initial admin user from secrets
        user = open('/run/secrets/admin-user').read().strip()
        pw = open('/run/secrets/admin-pass').read().strip()
        add_or_update_user_db(user, pw)
        logging.info(f"Seeded user `{user}`")

    except FileNotFoundError:
        logging.warning("Admin secrets not found. Skipping user seeding.")
    except Exception as e:
        logging.critical(f"FATAL: An error occurred during startup: {e}")
        # Ensure state is clean on failure to prevent routes from using stale/bad config
        app.state.config = None

    scheduler.start()
    yield
    # Shutdown
    scheduler.shutdown()

# --- FastAPI App ---
app = FastAPI(lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:51819", "http://0.0.0.0:51819"], # Be more specific in production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- API Routers ---
app.include_router(auth_router, prefix="/api", tags=["Authentication"])
app.include_router(api_router, prefix="/api", tags=["API"])
