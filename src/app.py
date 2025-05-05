from time import strftime, gmtime
from os import getloadavg, environ
from flask import Flask, jsonify, request, render_template
from flask_httpauth import HTTPBasicAuth, HTTPTokenAuth
from itsdangerous import TimedJSONWebSignatureSerializer as Serializer, BadSignature
from scheduler import scheduler
from peers import create_peer, delete_peer, list_peers, peer_stats
from flasgger import Swagger
from utils import get_server_pubkey
from db import init_db, add_or_update_user_db, verify_user_db


# -------------------------------------------------------------------
# Helpers for token generation/verification
# -------------------------------------------------------------------
def make_token_serializer(app):
    return Serializer(app.config['SECRET_KEY'], expires_in=1800)


def generate_token(s, username):
    return s.dumps({'user': username}).decode('utf-8')


def verify_token(s, token):
    try:
        data = s.loads(token)
        return data.get('user')
    except (BadSignature):
        return None


# -------------------------------------------------------------------
# Application factory
# -------------------------------------------------------------------
def create_app():
    # Create application and collect environment variables
    flask_app = Flask(__name__, instance_relative_config=True)
    flask_app.config['JSON_SORT_KEYS'] = False
    flask_app.config['WG_SERVER_PUBKEY'] = get_server_pubkey()
    flask_app.config['WG_ENDPOINT'] = environ.get('WG_ENDPOINT')

    # Start Swagger for API documentation
    Swagger(flask_app)

    # Start scheduler
    scheduler.init_app(flask_app)
    scheduler.start()

    # Initialize DB + (re)seed default admin user every launch
    with app.app_context():
        init_db()
        admin_user = environ.get('ADMIN_USER', 'admin')
        admin_pass = environ.get('ADMIN_PASS', 'changeme')
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
    @flask_app.route('/login', methods=['POST'])
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
            with open('/proc/uptime', 'r') as f:
                uptime_seconds = float(f.readline().split()[0])
            uptime_str = strftime("%H:%M:%S", gmtime(uptime_seconds))
            load_str = "{:.2f} {:.2f} {:.2f}".format(*getloadavg())
            return jsonify({"uptime": uptime_str, "load": load_str})
        except Exception as e:
            return jsonify({"error": str(e)}), 500

    @flask_app.route('/')
    @token_auth.login_required
    def serve_ui():
        return render_template('index.html',
                               server_public_key=app.config['WG_SERVER_PUBKEY'],
                               server_endpoint=app.config['WG_ENDPOINT'])

    return flask_app

# Create application instance at module level
app = create_app()
if __name__ == "__main__":
    app.run()
