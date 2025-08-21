from fastapi import APIRouter, Depends, HTTPException, status, Request
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm
from itsdangerous import SignatureExpired, BadSignature

from db import verify_user_db
from config import config

router = APIRouter()
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/api/login")


def generate_token(request: Request, username: str) -> str:
    """Generates a time-sensitive token for the user."""
    return request.app.state.config.ts.dumps({'user': username})


def verify_token(request: Request, token: str = Depends(oauth2_scheme)) -> str:
    """Verifies the provided token. Raises HTTPException on failure."""
    if not request.app.state.config.ts:
        raise HTTPException(status_code=503, detail="Authentication service not available due to startup error.")
    try:
        user = request.app.state.config.ts.loads(token, max_age=1800).get('user')
        if user is None:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Invalid token",
                headers={"WWW-Authenticate": "Bearer"},
            )
        return user
    except SignatureExpired:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token has expired",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except BadSignature:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token signature",
            headers={"WWW-Authenticate": "Bearer"},
        )


@router.post("/login")
async def login(request: Request, form_data: OAuth2PasswordRequestForm = Depends()):
    """Endpoint to authenticate a user and provide an access token."""
    if not verify_user_db(form_data.username, form_data.password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )
    access_token = generate_token(request, form_data.username)
    return {"access_token": access_token, "token_type": "bearer"}
