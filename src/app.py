from time import strftime, gmtime
from os import getloadavg, environ
from flask import Flask, jsonify, request
from flask_cors import CORS
from flask_httpauth import HTTPTokenAuth
from itsdangerous import SignatureExpired, URLSafeTimedSerializer as Serializer
from itsdangerous.exc import BadSignature
from scheduler import scheduler
from peers import create_peer, delete_peer, list_peers, peer_stats
from flasgger import Swagger
from utils import get_server_pubkey
from db import init_db, add_or_update_user_db, verify_user_db
from logging import basicConfig, DEBUG, error, warning

basicConfig(level=DEBUG)

def make_token_serializer(flask_app):
    return Serializer(flask_app.config['SECRET_KEY'], salt='auth-token')

def generate_token(s, username):
    return s.dumps({'user': username})

def verify_token(s, token, max_age=1800):
    try:
        return s.loads(token, max_age=max_age).get('user')
    except SignatureExpired as e:
        warning(f"Token expired at {e.date_signed}")
        return None
    except BadSignature as e:
        error(f"Invalid token signature: {e}")
        return None

def create_app():
    flask_app = Flask(__name__, instance_relative_config=True)
    flask_app.config['SECRET_KEY'] = environ['SECRET_KEY']
    flask_app.config['WG_SERVER_PUBKEY'] = get_server_pubkey()
    flask_app.config['WG_ENDPOINT'] = environ.get('WG_ENDPOINT')

    Swagger(flask_app)
    scheduler.init_app(flask_app)
    scheduler.start()

    CORS(
    app,
    resources={
        r"/api/*": {"origins": "*"},
        r"/serverinfo": {"origins": "*"}
    },
    methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type"],
    expose_headers=["Authorization"],
    supports_credentials=True
)

    # DB + seed admin user
    with flask_app.app_context():
        init_db()
        user = environ['ADMIN_USER']
        pw = environ['ADMIN_PASS']
        add_or_update_user_db(user, pw)
        flask_app.logger.info(f"Seeded user `{user}`")

    # Token-only auth
    token_auth = HTTPTokenAuth(scheme='Bearer')
    ts = make_token_serializer(flask_app)

    @token_auth.verify_token
    def verify_tok(token):
        flask_app.logger.debug(f"verify_token received: {token!r}")
        return verify_token(ts, token)

    # ——— Routes ———
    @flask_app.route('/api/login', methods=['POST'])
    def login():
        data = request.get_json() or {}
        u = data.get('username','').strip()
        p = data.get('password','')
        if not u or not p or not verify_user_db(u, p):
            return jsonify({'error':'invalid credentials'}), 401
        return jsonify({'token': generate_token(ts,u)})

    @flask_app.route('/api/peers/new', methods=['POST'])
    @token_auth.login_required
    def api_create_peer():
        days = (request.get_json() or {}).get('days_valid',7)
        return jsonify(create_peer(days))

    @flask_app.route('/api/peers/delete', methods=['POST'])
    @token_auth.login_required
    def api_delete_peer():
        key = (request.get_json() or {}).get('public_key','')
        return jsonify({'deleted': delete_peer(key)})

    @flask_app.route('/api/peers/list', methods=['GET'])
    @token_auth.login_required
    def api_list_peers():
        return jsonify(list_peers())

    @flask_app.route('/api/peers/stats', methods=['GET'])
    @token_auth.login_required
    def api_peer_stats():
        return jsonify(peer_stats())

    @flask_app.route('/serverinfo', methods=['GET'])
    @token_auth.login_required
    def server_info():
        try:
            with open('/proc/uptime') as f:
                up = float(f.readline().split()[0])
            return jsonify({
                'uptime': strftime("%H:%M:%S", gmtime(up)),
                'load': "{:.2f} {:.2f} {:.2f}".format(*getloadavg())
            })
        except Exception as e:
            return jsonify({'error':str(e)}), 500

    return flask_app

app = create_app()
if __name__=="__main__":
    app.run()