// static/js/main.js

document.addEventListener('DOMContentLoaded', function() {
  // Element refs & peer storage
  const peersTable       = document.querySelector('#peersTable tbody');
  const statsTable       = document.querySelector('#statsTable tbody');
  const addPeerBtn       = document.getElementById('addPeerBtn');
  const refreshStatsBtn  = document.getElementById('refreshStatsButton');
  const darkModeToggle   = document.getElementById('darkModeToggle');
  const darkModeMediaQ   = window.matchMedia('(prefers-color-scheme: dark)');

  const peersMap = new Map();  // public_key → peer object

  // Dark mode
  function updateDarkModeUI() {
    if (document.body.classList.contains('darkmode')) {
      darkModeToggle.textContent = '☀️ Light Mode';
    } else {
      darkModeToggle.textContent = '🌙 Dark Mode';
    }
  }
  function applyPreferredMode() {
    if (darkModeMediaQ.matches) document.body.classList.add('darkmode');
    else                          document.body.classList.remove('darkmode');
    updateDarkModeUI();
  }
  darkModeMediaQ.addEventListener('change', applyPreferredMode);
  darkModeToggle.addEventListener('click', () => {
    document.body.classList.toggle('darkmode');
    updateDarkModeUI();
  });

  // Fetch & render peers
  function loadPeers() {
    fetch('/api/peers/list')
      .then(r => r.json())
      .then(peers => {
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
      });
  }

  window.deletePeer = function(pubKey) {
    fetch('/api/peers/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ public_key: pubKey })
    })
    .then(() => loadPeers());
  };

  // QR code builder
  function generateQRCode(peer) {
    const el = document.getElementById(`qrcode-${peer.public_key}`);
    el.innerHTML = '';

    const conf = `[Interface]
PrivateKey = ${peer.private_key}
Address    = ${peer.ipv4_address}/32, ${peer.ipv6_address}/128
DNS        = ${WG_DNS}

[Peer]
PublicKey           = ${WG_SERVER_PUBKEY}
Endpoint            = ${WG_ENDPOINT}
AllowedIPs          = 0.0.0.0/0, ::/0
PersistentKeepalive = ${WG_KEEPALIVE}
`;
    new QRCode(el, {
      text: conf,
      width: 160,
      height: 160,
      correctLevel: QRCode.CorrectLevel.H
    });
  }

  // Download .conf
  window.downloadConfig = function(pubKey) {
    const peer = peersMap.get(pubKey);
    if (!peer) return alert('Peer data not available yet.');

    const conf = `[Interface]
PrivateKey = ${peer.private_key}
Address    = ${peer.ipv4_address}/32, ${peer.ipv6_address}/128
DNS        = ${WG_DNS}

[Peer]
PublicKey           = ${WG_SERVER_PUBKEY}
Endpoint            = ${WG_ENDPOINT}
AllowedIPs          = 0.0.0.0/0, ::/0
PersistentKeepalive = ${WG_KEEPALIVE}
`;

    const blob = new Blob([conf], { type: 'text/plain' });
    const url  = URL.createObjectURL(blob);
    const a    = document.createElement('a');
    a.href       = url;
    a.download   = `wg-peer-${peer.ipv4_address}.conf`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  // Fetch & render stats
  function loadStats() {
    fetch('/api/peers/stats')
      .then(r => r.json())
      .then(stats => {
        statsTable.innerHTML = '';
        const now = Math.floor(Date.now() / 1000);
        let totalRX = 0, totalTX = 0;

        stats.forEach(s => {
          const ago = s.last_handshake_time ? now - s.last_handshake_time : Infinity;
          let cls = ago < 60 ? 'good' : ago < 300 ? 'warn' : 'stale';

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
      });
  }

  // Charts
  let rxChart, txChart, labels = [], rxData = [], txData = [];
  function initCharts() {
    const ctxRx = document.getElementById('rxChart').getContext('2d');
    const ctxTx = document.getElementById('txChart').getContext('2d');

    rxChart = new Chart(ctxRx, {
      type: 'line',
      data: { labels: [], datasets: [{ label:'RX (MB)', data: [], borderColor:'blue', backgroundColor:'lightblue' }]},
      options: { responsive: true }
    });
    txChart = new Chart(ctxTx, {
      type: 'line',
      data: { labels: [], datasets: [{ label:'TX (MB)', data: [], borderColor:'green', backgroundColor:'lightgreen' }]},
      options: { responsive: true }
    });
  }

  function updateCharts(rx, tx) {
    const now = new Date().toLocaleTimeString();
    labels.push(now); rxData.push((rx/1024/1024).toFixed(2)); txData.push((tx/1024/1024).toFixed(2));
    if (labels.length > 20) { labels.shift(); rxData.shift(); txData.shift(); }
    rxChart.data.labels = labels;
    rxChart.data.datasets[0].data = rxData;
    rxChart.update();
    txChart.data.labels = labels;
    txChart.data.datasets[0].data = txData;
    txChart.update();
  }

  // Server info
  function loadServerInfo() {
    fetch('/serverinfo')
      .then(r => r.json())
      .then(d => {
        document.getElementById('uptimeInfo').innerText  = `Uptime: ${d.uptime}`;
        document.getElementById('loadAvgInfo').innerText = `Load Average: ${d.load}`;
      })
      .catch(_ => {
        document.getElementById('uptimeInfo').innerText = 'Uptime: N/A';
        document.getElementById('loadAvgInfo').innerText = 'Load Average: N/A';
      });
  }

  // Event hookups
  addPeerBtn.addEventListener('click', () => {
    fetch('/api/peers/new', {
      method: 'POST',
      headers: { 'Content-Type':'application/json' },
      body: JSON.stringify({ days_valid:7 })
    })
    .then(() => loadPeers());
  });
  refreshStatsBtn.addEventListener('click', loadStats);

  // Init everything
  applyPreferredMode();
  initCharts();
  loadPeers();
  loadStats();
  setInterval(loadStats, 10000);
  setInterval(loadServerInfo, 60000);
});