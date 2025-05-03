document.addEventListener('DOMContentLoaded', function() {
  // Element references
  const peersTable = document.getElementById('peersTable').querySelector('tbody');
  const statsTable = document.getElementById('statsTable').querySelector('tbody');
  const addPeerBtn = document.getElementById('addPeerBtn');
  const refreshStatsBtn = document.getElementById('refreshStatsButton');
  const darkModeToggle = document.getElementById('darkModeToggle');
  const darkModeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

  // Dark mode UI updates
  function updateDarkModeUI() {
    if (document.body.classList.contains('darkmode')) {
      darkModeToggle.textContent = '☀️ Light Mode';
    } else {
      darkModeToggle.textContent = '🌙 Dark Mode';
    }
  }

  function applyPreferredMode() {
    if (darkModeMediaQuery.matches) {
      document.body.classList.add('darkmode');
    } else {
      document.body.classList.remove('darkmode');
    }
    updateDarkModeUI();
  }

  // Listen for OS-level changes
  darkModeMediaQuery.addEventListener('change', applyPreferredMode);

  // Manual toggle handler
  darkModeToggle.addEventListener('click', () => {
    document.body.classList.toggle('darkmode');
    updateDarkModeUI();
  });

  // Chart variables
  let rxChart, txChart;
  let rxData = [];
  let txData = [];
  let labels = [];

  function loadPeers() {
     fetch('/api/peers/list')
       .then(r => r.json())
       .then(peers => {
         const tbody = document.querySelector('#peersTable tbody');
         tbody.innerHTML = '';
         peers.forEach(peer => {
           const row = document.createElement('tr');
           row.innerHTML = `
             <td>${peer.public_key}</td>
             <td>${peer.ipv4_address}</td>
             <td>${peer.ipv6_address}</td>
             <td>${peer.expires_at?.split('T')[0] || 'N/A'}</td>
             <td><div id="qrcode-${peer.public_key}" class="qrcode"></div></td>
             <td>
               <button onclick="deletePeer('${peer.public_key}')">
                 Delete
               </button>
             </td>`;
           tbody.appendChild(row);
           generateQRCode(peer);  // ← now pass the full peer object
         });
       });
  }


  // Delete a peer
  function deletePeer(publicKey) {
    fetch('/api/peers/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ public_key: publicKey })
    })
    .then(res => res.json())
    .then(() => loadPeers());
  }

  function generateQRCode(peer) {
      const qrDiv = document.getElementById(`qrcode-${peer.public_key}`);
      qrDiv.innerHTML = '';

// assemble a full WireGuard config
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

      new QRCode(qrDiv, {
        text: conf,
        width: 160,
        height: 160,
        correctLevel: QRCode.CorrectLevel.H
      });
  }

  // Load stats
  function loadStats() {
    fetch('/api/peers/stats')
      .then(res => res.json())
      .then(stats => {
        statsTable.innerHTML = '';
        const now = Math.floor(Date.now() / 1000);
        let totalRX = 0, totalTX = 0;

        stats.forEach(stat => {
          const lastHandshakeAgo = stat.last_handshake_time ? now - stat.last_handshake_time : Infinity;
          let hsClass = 'stale';
          if (lastHandshakeAgo < 60) hsClass = 'good';
          else if (lastHandshakeAgo < 300) hsClass = 'warn';

          const row = document.createElement('tr');
          row.innerHTML = `
            <td>${stat.public_key}</td>
            <td class="${hsClass}">${lastHandshakeAgo}</td>
            <td>${(stat.rx_bytes / 1024 / 1024).toFixed(2)}</td>
            <td>${(stat.tx_bytes / 1024 / 1024).toFixed(2)}</td>
          `;
          statsTable.appendChild(row);

          totalRX += stat.rx_bytes;
          totalTX += stat.tx_bytes;
        });

        updateCharts(totalRX, totalTX);
      });
  }

  // Chart update logic
  function updateCharts(rx, tx) {
    const now = new Date().toLocaleTimeString();
    labels.push(now);
    rxData.push((rx / 1024 / 1024).toFixed(2));
    txData.push((tx / 1024 / 1024).toFixed(2));

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

  // Initialize Chart.js charts
  function initCharts() {
    const ctxRx = document.getElementById('rxChart').getContext('2d');
    const ctxTx = document.getElementById('txChart').getContext('2d');

    rxChart = new Chart(ctxRx, {
      type: 'line', data: { labels: [], datasets: [{ label: 'RX (MB)', data: [], borderColor: 'blue', backgroundColor: 'lightblue' }]},
      options: { responsive: true }
    });

    txChart = new Chart(ctxTx, {
      type: 'line', data: { labels: [], datasets: [{ label: 'TX (MB)', data: [], borderColor: 'green', backgroundColor: 'lightgreen' }]},
      options: { responsive: true }
    });
  }

  // Load server info
  function loadServerInfo() {
    fetch('/serverinfo')
      .then(res => res.json())
      .then(data => {
        document.getElementById('uptimeInfo').innerText = `Uptime: ${data.uptime}`;
        document.getElementById('loadAvgInfo').innerText = `Load Average: ${data.load}`;
      })
      .catch(() => {
        document.getElementById('uptimeInfo').innerText = 'Uptime: N/A';
        document.getElementById('loadAvgInfo').innerText = 'Load Average: N/A';
      });
  }

  // Event listeners
  addPeerBtn.addEventListener('click', () => {
    fetch('/api/peers/new', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ days_valid: 7 })
    })
    .then(res => res.json())
    .then(() => loadPeers());
  });

  refreshStatsBtn.addEventListener('click', loadStats);
  window.deletePeer = deletePeer;

  // Kick things off
  applyPreferredMode();
  initCharts();
  loadPeers();
  loadStats();
  setInterval(loadStats, 10000);
  setInterval(loadServerInfo, 60000);
});