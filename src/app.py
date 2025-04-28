from datetime import time
import os
from flask import Flask, jsonify, request, render_template
from scheduler import scheduler
from peers import create_peer, delete_peer, list_peers, peer_stats
from flasgger import Swagger

app = Flask(__name__)
Swagger(app)

app.config['JSON_SORT_KEYS'] = False

scheduler.init_app(app)
scheduler.start()

@app.route('/api/peers/new', methods=['POST'])
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

@app.route('/api/peers/delete', methods=['POST'])
def api_delete_peer():
    """
    Delete a WireGuard peer by PublicKey
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

@app.route('/api/peers/list', methods=['GET'])
def api_list_peers():
    """
    List all WireGuard peers
    ---
    responses:
      200:
        description: List of peers
    """
    return jsonify(list_peers())

@app.route('/api/peers/stats', methods=['GET'])
def api_peer_stats():
    """
    Live WireGuard peer stats
    ---
    responses:
      200:
        description: List of active peers with traffic stats
    """
    return jsonify(peer_stats())

@app.route('/serverinfo', methods=['GET'])
def server_info():
    try:
        with open('/proc/uptime', 'r') as f:
            uptime_seconds = float(f.readline().split()[0])
            uptime_str = time.strftime("%H:%M:%S", time.gmtime(uptime_seconds))

        load_avg = os.getloadavg()  # (1m, 5m, 15m)
        load_str = "{:.2f} {:.2f} {:.2f}".format(*load_avg)

        return jsonify({
            "uptime": uptime_str,
            "load": load_str
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/')
def serve_ui():
    return render_template('index.html')

if __name__ == "__main__":
    app.run()