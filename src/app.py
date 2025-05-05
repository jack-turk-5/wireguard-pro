from time import strftime, gmtime
from os import getloadavg, environ
from flask import Flask, jsonify, request, render_template
from flask_httpauth import HTTPBasicAuth, HTTPTokenAuth
from itsdangerous import URLSafeTimedSerializer as Serializer
from itsdangerous.exc import BadSignature
from scheduler import scheduler
from peers import create_peer, delete_peer, list_peers, peer_stats
from flasgger import Swagger
from utils import get_server_pubkey
from db import init_db, add_or_update_user_db, verify_user_db
from flask_cors import CORS
import logging

logging.basicConfig(level=logging.DEBUG)

# -------------------------------------------------------------------
# Helpers for token generation/verification
# -------------------------------------------------------------------
def make_token_serializer(app):
    return Serializer(app.config['SECRET_KEY'], salt='auth-token')

def generate_token(s, username):
    return s.dumps({'user': username})

def verify_token(s, token, max_age=1800):
    try:
        data = s.loads(token, max_age=max_age)
        return data.get('user')
    except BadSignature:
        return None

# -------------------------------------------------------------------
# Application factory
# -------------------------------------------------------------------
def create_app():
    flask_app = Flask(__name__, instance_relative_config=True)
    flask_app.config['SECRET_KEY']      = environ.get('SECRET_KEY', 'replace_this!')
    flask_app.config['JSON_SORT_KEYS']  = False
    flask_app.config['WG_SERVER_PUBKEY']= get_server_pubkey()
    flask_app.config['WG_ENDPOINT']     = environ.get('WG_ENDPOINT')

    Swagger(flask_app)
    scheduler.init_app(flask_app)
    scheduler.start()

    # Initialize DB + seed admin user if missing
    with flask_app.app_context():
        init_db()
        admin_user = environ.get('ADMIN_USER', 'admin').strip()
        admin_pass = environ.get('ADMIN_PASS', 'changeme').strip()
        if not admin_user or not admin_pass:
            raise RuntimeError("ADMIN_USER and ADMIN_PASS must both be set")
        add_or_update_user_db(admin_user, admin_pass)
        flask_app.logger.info(f"🛡 Ensured user `{admin_user}` exists")

    # Enable CORS for login and API endpoints
    CORS(flask_app,
         supports_credentials=True,
         resources={
             r"/login": {"origins": "https://vpn.jackturk.dev"},
             r"/api/*": {"origins": "https://vpn.jackturk.dev"},
             r"/serverinfo": {"origins": "https://vpn.jackturk.dev"},
         })

    # Auth setup
    basic_auth = HTTPBasicAuth()
    token_auth = HTTPTokenAuth(scheme='Bearer')
    ts = make_token_serializer(flask_app)

    @basic_auth.verify_password
    def verify_pw(username, password):
        flask_app.logger.debug(f"verify_pw called, username={username!r}, password={password!r}")
        return verify_user_db(username, password)

    @token_auth.verify_token
    def verify_tok(token):
        return verify_token(ts, token)

    # -------------------------------------------------------------------
    # Routes
    # -------------------------------------------------------------------
    @flask_app.route('/login', methods=['POST'])
    @basic_auth.login_required
    def login():
        token = generate_token(ts, basic_auth.current_user())
        return jsonify({'token': token})

    @flask_app.route('/api/peers/new', methods=['POST'])
    @token_auth.login_required
    def api_create_peer():
        data = request.get_json()
        peer = create_peer(data.get('days_valid', 7))
        return jsonify(peer)

    @flask_app.route('/api/peers/delete', methods=['POST'])
    @token_auth.login_required
    def api_delete_peer():
        data = request.get_json()
        success = delete_peer(data['public_key'])
        return jsonify({"deleted": success})

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
                uptime_seconds = float(f.readline().split()[0])
            uptime_str = strftime("%H:%M:%S", gmtime(uptime_seconds))
            load_str   = "{:.2f} {:.2f} {:.2f}".format(*getloadavg())
            return jsonify({"uptime": uptime_str, "load": load_str})
        except Exception as e:
            return jsonify({"error": str(e)}), 500

    @flask_app.route('/')
    def serve_ui():
        return render_template('index.html',
                               server_public_key=flask_app.config['WG_SERVER_PUBKEY'],
                               server_endpoint=flask_app.config['WG_ENDPOINT'])

    return flask_app

# Create application instance at module level
app = create_app()
if __name__ == "__main__":
    app.run()