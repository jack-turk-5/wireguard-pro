document.addEventListener('DOMContentLoaded', function() {
  // Element refs & peer storage
  const peersTable       = document.querySelector('#peersTable tbody');
  const statsTable       = document.querySelector('#statsTable tbody');
  const addPeerBtn       = document.getElementById('addPeerBtn');
  const refreshStatsBtn  = document.getElementById('refreshStatsButton');
  const darkModeToggle   = document.getElementById('darkModeToggle');
  const darkModeMediaQ   = window.matchMedia('(prefers-color-scheme: dark)');

  const peersMap = new Map();  // public_key → peer object
  let apiToken = null;         // once-set bearer token

  // -------------------------
  // 1) Single‑sign‑in wrapper
  // -------------------------
  async function doLogin() {
    while (true) {
      const user = prompt("Username:");
      const pass = prompt("Password:");
      const resp = await fetch('/login', {
        method: 'POST',
        headers: { 'Authorization':'Basic '+btoa(user+':'+pass) }
      });
      if (resp.ok) {
        const { token } = await resp.json();
        apiToken = token;
        break;    // success → exit loop
      }
      alert("Login failed, please try again.");  // failure → repeat
    }
  }
    const { token } = await resp.json();
    apiToken = token;
  }

  async function authFetch(url, opts = {}) {
    if (!apiToken) {
      await doLogin();
    }
    opts.headers = {
      ...opts.headers,
      'Authorization': 'Bearer ' + apiToken,
      'Content-Type': 'application/json'
    };
    let res = await fetch(url, opts);
    if (res.status === 401) {
      // token expired or invalid → re-login once
      apiToken = null;
      await doLogin();
      opts.headers['Authorization'] = 'Bearer ' + apiToken;
      res = await fetch(url, opts);
    }
    return res;
  }

  // -------------------------
  // 2) Dark‑mode logic (unchanged)
  // -------------------------
  function updateDarkModeUI() {
    darkModeToggle.textContent = document.body.classList.contains('darkmode')
      ? '☀️ Light Mode'
      : '🌙 Dark Mode';
  }
  function applyPreferredMode() {
    document.body.classList.toggle('darkmode', darkModeMediaQ.matches);
    updateDarkModeUI();
  }
  darkModeMediaQ.addEventListener('change', applyPreferredMode);
  darkModeToggle.addEventListener('click', () => {
    document.body.classList.toggle('darkmode');
    updateDarkModeUI();
  });

  // -------------------------
  // 3) Peers fetching & rendering
  // -------------------------
  async function loadPeers() {
    const resp = await authFetch('/api/peers/list');
    const peers = await resp.json();
    peersMap.clear();
    peersTable.innerHTML = '';
    peers.forEach(peer => {
      peersMap.set(peer.public_key, peer);
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${peer.public_key}</td>
        <td>${peer.ipv4_address}</td>
        <td>${peer.ipv6_address}</td>
        <td>${peer.expires_at?.split('T')[0] || 'N/A'}</td>
        <td><div id="qrcode-${peer.public_key}" class="qrcode"></div></td>
        <td class="actions">
           <div class="action-container">
             <button class="action-btn" onclick="downloadConfig('${peer.public_key}')">Download</button>
             <button class="action-btn" onclick="deletePeer('${peer.public_key}')">Delete</button>
           </div>
         </td>`;
      peersTable.appendChild(tr);
      generateQRCode(peer);
    });
  }

  window.deletePeer = async function(pubKey) {
    await authFetch('/api/peers/delete', {
      method: 'POST',
      body: JSON.stringify({ public_key: pubKey })
    });
    loadPeers();
  };

  // -------------------------
  // 4) QR code builder & download
  // -------------------------
  function generateQRCode(peer) {
    const el = document.getElementById(`qrcode-${peer.public_key}`);
    el.innerHTML = '';
    const conf = `[Interface]
PrivateKey = ${peer.private_key}
Address = ${peer.ipv4_address}/32, ${peer.ipv6_address}/128
DNS = ${WG_DNS}

[Peer]
PublicKey = ${WG_SERVER_PUBKEY}
Endpoint = ${WG_ENDPOINT}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = ${WG_KEEPALIVE}
`;
    new QRCode(el, {
      text: conf,
      width: 160,
      height: 160,
      correctLevel: QRCode.CorrectLevel.H
    });
  }

  window.downloadConfig = function(pubKey) {
    const peer = peersMap.get(pubKey);
    if (!peer) return alert('Peer data not available yet.');

    const conf = `[Interface]
PrivateKey = ${peer.private_key}
Address = ${peer.ipv4_address}/32, ${peer.ipv6_address}/128
DNS = ${WG_DNS}

[Peer]
PublicKey = ${WG_SERVER_PUBKEY}
Endpoint = ${WG_ENDPOINT}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = ${WG_KEEPALIVE}
`;
    const blob = new Blob([conf], { type: 'text/plain' });
    const a    = document.createElement('a');
    a.href       = URL.createObjectURL(blob);
    a.download   = `wg-peer-${peer.ipv4_address}.conf`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(a.href);
  };

  // -------------------------
  // 5) Stats fetching & rendering
  // -------------------------
  async function loadStats() {
    const resp = await authFetch('/api/peers/stats');
    const stats = await resp.json();
    statsTable.innerHTML = '';
    const now = Math.floor(Date.now() / 1000);
    let totalRX = 0, totalTX = 0;

    stats.forEach(s => {
      const ago = s.last_handshake_time ? now - s.last_handshake_time : Infinity;
      const cls = ago < 60 ? 'good' : ago < 300 ? 'warn' : 'stale';

      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${s.public_key}</td>
        <td class="${cls}">${ago}</td>
        <td>${(s.rx_bytes/1024/1024).toFixed(2)}</td>
        <td>${(s.tx_bytes/1024/1024).toFixed(2)}</td>
      `;
      statsTable.appendChild(tr);
      totalRX += s.rx_bytes;
      totalTX += s.tx_bytes;
    });

    updateCharts(totalRX, totalTX);
  }

  // -------------------------
  // 6) Charts: init & update
  // -------------------------
  let rxChart, txChart, labels = [], rxData = [], txData = [];

  function initCharts() {
    const ctxRx = document.getElementById('rxChart').getContext('2d');
    const ctxTx = document.getElementById('txChart').getContext('2d');

    const commonOptions = {
      responsive: true,
      scales: {
        y: { beginAtZero: true }
      }
    };

    rxChart = new Chart(ctxRx, {
      type: 'line',
      data: { labels: [], datasets:[{ label:'RX (MB)', data:[], fill:false }]},
      options: commonOptions
    });

    txChart = new Chart(ctxTx, {
      type: 'line',
      data: { labels: [], datasets:[{ label:'TX (MB)', data:[], fill:false }]},
      options: commonOptions
    });
  }

  function updateCharts(rx, tx) {
    labels.push(new Date().toLocaleTimeString());
    rxData.push( parseFloat((rx/1024/1024).toFixed(2)) );
    txData.push( parseFloat((tx/1024/1024).toFixed(2)) );

    if (labels.length > 20) {
      labels.shift(); rxData.shift(); txData.shift();
    }

    rxChart.data.labels = labels;
    rxChart.data.datasets[0].data = rxData;
    rxChart.update();

    txChart.data.labels = labels;
    txChart.data.datasets[0].data = txData;
    txChart.update();
  }

  // -------------------------
  // 7) Server info
  // -------------------------
  async function loadServerInfo() {
    try {
      const resp = await authFetch('/serverinfo');
      const d = await resp.json();
      document.getElementById('uptimeInfo').innerText  = `Uptime: ${d.uptime}`;
      document.getElementById('loadAvgInfo').innerText = `Load Average: ${d.load}`;
    } catch {
      document.getElementById('uptimeInfo').innerText  = 'Uptime: N/A';
      document.getElementById('loadAvgInfo').innerText = 'Load Average: N/A';
    }
  }

  // -------------------------
  // 8) Event hookups
  // -------------------------
  addPeerBtn.addEventListener('click', async () => {
    await authFetch('/api/peers/new', {
      method: 'POST',
      body: JSON.stringify({ days_valid: 7 })
    });
    loadPeers();
  });
  refreshStatsBtn.addEventListener('click', loadStats);

  // -------------------------
  // Init everything
  // -------------------------
  applyPreferredMode();
  initCharts();
  loadPeers();
  loadStats();
  setInterval(loadStats, 10000);
  setInterval(loadServerInfo, 60000);
});