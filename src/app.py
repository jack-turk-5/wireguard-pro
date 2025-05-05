from time import strftime, gmtime
from os import getloadavg, environ
from flask import Flask, jsonify, request, render_template
from scheduler import scheduler
from peers import create_peer, delete_peer, list_peers, peer_stats
from flasgger import Swagger
from utils import get_server_pubkey
from db import init_db

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

    # Initiate and inject Db into application context
    with flask_app.app_context():
        init_db()

    @flask_app.route('/api/peers/new', methods=['POST'])
    def api_create_peer():
        """
        Create a new WireGuard peer
        ---
        parameters:
          - name: days_valid
            in: json
            type: integer
            required: false
            description: Days before expiration
        responses:
          200:
            description: Peer created successfully
        """
        data = request.get_json()
        peer = create_peer(data.get('days_valid', 7))
        return jsonify(peer)

    @flask_app.route('/api/peers/delete', methods=['POST'])
    def api_delete_peer():
        """
        Delete a WireGuard peer by Public Key
        ---
        parameters:
          - name: public_key
            in: json
            type: string
            required: true
            description: Public key to remove
        responses:
          200:
            description: Peer deleted
        """
        data = request.get_json()
        success = delete_peer(data['public_key'])
        return jsonify({"deleted": success})

    @flask_app.route('/api/peers/list', methods=['GET'])
    def api_list_peers():
        """
        List all WireGuard peers
        ---
        responses:
          200:
            description: List of peers
        """
        return jsonify(list_peers())

    @flask_app.route('/api/peers/stats', methods=['GET'])
    def api_peer_stats():
        """
        Live WireGuard peer stats
        ---
        responses:
          200:
            description: List of active peers with traffic stats
        """
        return jsonify(peer_stats())

    @flask_app.route('/serverinfo', methods=['GET'])
    def server_info():
        try:
            with open('/proc/uptime', 'r') as f:
                first_field, *_ = f.readline().split()
                uptime_seconds = float(first_field)
                uptime_str = strftime("%H:%M:%S", gmtime(uptime_seconds))

            load_avg = getloadavg()  # (1m, 5m, 15m)
            load_str = "{:.2f} {:.2f} {:.2f}".format(*load_avg)

            return jsonify({
                "uptime": uptime_str,
                "load": load_str
            })
        except Exception as e:
            return jsonify({"error": str(e)}), 500

    @flask_app.route('/')
    def serve_ui():
        return render_template(
            'index.html',
            server_public_key=flask_app.config['WG_SERVER_PUBKEY'],
            server_endpoint=flask_app.config['WG_ENDPOINT'])

    return flask_app

# Create application instance at module level
app = create_app()
if __name__ == "__main__":
    # Run app, entrypoint logic in container/entrypoint.py
    app.run()