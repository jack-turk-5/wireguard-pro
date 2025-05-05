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


# -------------------------------------------------------------------
# Helpers for token generation/verification
# -------------------------------------------------------------------
def make_token_serializer(app):
    # URLSafeTimedSerializer takes only secret_key and salt (optional)
    return Serializer(app.config['SECRET_KEY'], salt='auth-token')

def generate_token(s, username):
    # URLSafeTimedSerializer.dumps returns a string
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
    app = Flask(__name__, instance_relative_config=True)
    app.config['SECRET_KEY'] = environ.get('SECRET_KEY', 'replace_this!')
    app.config['JSON_SORT_KEYS'] = False
    app.config['WG_SERVER_PUBKEY'] = get_server_pubkey()
    app.config['WG_ENDPOINT'] = environ.get('WG_ENDPOINT')

    Swagger(app)
    scheduler.init_app(app)
    scheduler.start()

    # Initialize DB + (re)seed default admin user every launch
    with app.app_context():
        init_db()
        admin_user = environ.get('ADMIN_USER').strip()
        admin_pass = environ.get('ADMIN_PASS').strip()
        add_or_update_user_db(admin_user, admin_pass)
        print(f"🛡 Ensured user `{admin_user}` with provided password.")

    # Auth objects
    basic_auth = HTTPBasicAuth()
    token_auth = HTTPTokenAuth(scheme='Bearer')
    ts = make_token_serializer(app)

    # BasicAuth verify for /login
    @basic_auth.verify_password
    def verify_pw(username, password):
        return verify_user_db(username, password)

    # TokenAuth verify for API
    @token_auth.verify_token
    def verify_tok(token):
        user = verify_token(ts, token)
        return user

    # -------------------------------------------------------------------
    # Routes
    # -------------------------------------------------------------------
    @app.route('/login', methods=['POST'])
    @basic_auth.login_required
    def login():
        """
        ---
        post:
          summary: Obtain a bearer token
          security:
            - basicAuth: []
          responses:
            200:
              description: JSON with token
        """
        token = generate_token(ts, basic_auth.current_user())
        return jsonify({'token': token})

    @app.route('/api/peers/new', methods=['POST'])
    @token_auth.login_required
    def api_create_peer():
        data = request.get_json()
        peer = create_peer(data.get('days_valid', 7))
        return jsonify(peer)

    @app.route('/api/peers/delete', methods=['POST'])
    @token_auth.login_required
    def api_delete_peer():
        data = request.get_json()
        success = delete_peer(data['public_key'])
        return jsonify({"deleted": success})

    @app.route('/api/peers/list', methods=['GET'])
    @token_auth.login_required
    def api_list_peers():
        return jsonify(list_peers())

    @app.route('/api/peers/stats', methods=['GET'])
    @token_auth.login_required
    def api_peer_stats():
        return jsonify(peer_stats())

    @app.route('/serverinfo', methods=['GET'])
    @token_auth.login_required
    def server_info():
        try:
            with open('/proc/uptime', 'r') as f:
                uptime_seconds = float(f.readline().split()[0])
            uptime_str = strftime("%H:%M:%S", gmtime(uptime_seconds))
            load_str = "{:.2f} {:.2f} {:.2f}".format(*getloadavg())
            return jsonify({"uptime": uptime_str, "load": load_str})
        except Exception as e:
            return jsonify({"error": str(e)}), 500

    @app.route('/')
    def serve_ui():
        return render_template('index.html',
                               server_public_key=app.config['WG_SERVER_PUBKEY'],
                               server_endpoint=app.config['WG_ENDPOINT'])

    return app


app = create_app()

if __name__ == "__main__":
    app.run()
