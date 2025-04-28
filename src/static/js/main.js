document.addEventListener('DOMContentLoaded', function() {
    const peersTable = document.getElementById('peersTable').querySelector('tbody');
    const statsTable = document.getElementById('statsTable').querySelector('tbody');
    const addPeerBtn = document.getElementById('addPeerBtn');
    const refreshStatsBtn = document.getElementById('refreshStatsBtn');
    const darkModeToggle = document.getElementById('darkModeToggle');
  
    let rxChart, txChart;
    let rxData = [];
    let txData = [];
    let labels = [];
  
    function loadPeers() {
      fetch('/api/peers/list')
        .then(response => response.json())
        .then(peers => {
          peersTable.innerHTML = '';
          peers.forEach(peer => {
            const row = document.createElement('tr');
            row.innerHTML = `
              <td>${peer.public_key}</td>
              <td>${peer.ipv4_address}</td>
              <td>${peer.ipv6_address}</td>
              <td>${peer.expires_at ? peer.expires_at.split('T')[0] : 'N/A'}</td>
              <td><div id="qrcode-${peer.public_key}" class="qrcode"></div></td>
              <td><button onclick="deletePeer('${peer.public_key}')">Delete</button></td>
            `;
            peersTable.appendChild(row);
            generateQRCode(peer.public_key);
          });
        });
    }
  
    function deletePeer(publicKey) {
      fetch('/api/peers/delete', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ public_key: publicKey })
      })
      .then(response => response.json())
      .then(() => loadPeers());
    }
  
    function generateQRCode(publicKey) {
      const qrDiv = document.getElementById(`qrcode-${publicKey}`);
      qrDiv.innerHTML = ''; // Clear previous
      new QRCode(qrDiv, {
        text: `PublicKey: ${publicKey}`,
        width: 64,
        height: 64
      });
    }
  
    function loadStats() {
      fetch('/api/peers/stats')
        .then(response => response.json())
        .then(stats => {
          statsTable.innerHTML = '';
          const now = Math.floor(Date.now() / 1000);
          let totalRX = 0;
          let totalTX = 0;
          stats.forEach(stat => {
            const lastHandshakeAgo = stat.last_handshake_time ? now - stat.last_handshake_time : Infinity;
            const row = document.createElement('tr');
            let handshakeClass = 'stale';
            if (lastHandshakeAgo < 60) handshakeClass = 'good';
            else if (lastHandshakeAgo < 300) handshakeClass = 'warn';
  
            row.innerHTML = `
              <td>${stat.public_key}</td>
              <td class="${handshakeClass}">${lastHandshakeAgo}</td>
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
  
    function updateCharts(rx, tx) {
      const now = new Date().toLocaleTimeString();
      labels.push(now);
      rxData.push((rx / 1024 / 1024).toFixed(2));
      txData.push((tx / 1024 / 1024).toFixed(2));
  
      if (labels.length > 20) {
        labels.shift();
        rxData.shift();
        txData.shift();
      }
  
      rxChart.data.labels = labels;
      rxChart.data.datasets[0].data = rxData;
      rxChart.update();
  
      txChart.data.labels = labels;
      txChart.data.datasets[0].data = txData;
      txChart.update();
    }
  
    function initCharts() {
      const ctxRx = document.getElementById('rxChart').getContext('2d');
      const ctxTx = document.getElementById('txChart').getContext('2d');
  
      rxChart = new Chart(ctxRx, {
        type: 'line',
        data: {
          labels: [],
          datasets: [{
            label: 'RX (MB)',
            backgroundColor: 'lightblue',
            borderColor: 'blue',
            data: []
          }]
        },
        options: { responsive: true }
      });
  
      txChart = new Chart(ctxTx, {
        type: 'line',
        data: {
          labels: [],
          datasets: [{
            label: 'TX (MB)',
            backgroundColor: 'lightgreen',
            borderColor: 'green',
            data: []
          }]
        },
        options: { responsive: true }
      });
    }
  
    function toggleDarkMode() {
      document.body.classList.toggle('darkmode');
    }
  
    function loadServerInfo() {
      fetch('/serverinfo') // This endpoint will need to be created if you want real data
        .then(response => response.json())
        .then(data => {
          document.getElementById('uptimeInfo').innerText = `Uptime: ${data.uptime}`;
          document.getElementById('loadAvgInfo').innerText = `Load Average: ${data.load}`;
        })
        .catch(() => {
          document.getElementById('uptimeInfo').innerText = `Uptime: N/A`;
          document.getElementById('loadAvgInfo').innerText = `Load Average: N/A`;
        });
    }
  
    darkModeToggle.addEventListener('click', toggleDarkMode);
    addPeerBtn.addEventListener('click', function() {
      fetch('/api/peers/new', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ days_valid: 7 })
      })
      .then(response => response.json())
      .then(() => loadPeers());
    });
    refreshStatsBtn.addEventListener('click', loadStats);
  
    window.deletePeer = deletePeer;
  
    initCharts();
    loadPeers();
    loadStats();
    setInterval(loadStats, 10000);
    setInterval(loadServerInfo, 60000);
  });  