document.addEventListener('DOMContentLoaded', () => {
  initCopyButton();
  initLiveDemo();
});

// Copy button with toast notification
function initCopyButton() {
  const copyBtn = document.getElementById('copyBtn');
  const toast = document.getElementById('toast');
  const installCmd = "curl -sSL https://raw.githubusercontent.com/Woffluon/Statix/main/deploy/install.sh | sudo bash";

  if (!copyBtn) return;

  copyBtn.addEventListener('click', () => {
    navigator.clipboard.writeText(installCmd).then(() => {
      // Toast feedback
      toast.classList.add('show');
      copyBtn.innerHTML = `
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
        Copied!
      `;

      setTimeout(() => {
        toast.classList.remove('show');
        copyBtn.innerHTML = `
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
          Copy
        `;
      }, 2500);
    });
  });
}

// Live Simulated Statix Dashboard Demo
function initLiveDemo() {
  const canvas = document.getElementById('demoCanvas');
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  let cpuData = Array(30).fill(15);
  let ramData = Array(30).fill(42);

  // Tab switching
  const tabs = document.querySelectorAll('.tab-btn');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      const view = tab.getAttribute('data-view');
      document.querySelectorAll('.demo-view').forEach(v => v.style.display = 'none');
      document.getElementById('view-' + view).style.display = 'block';
    });
  });

  // Simulated metrics tick loop
  function tick() {
    // Generate realistic fluctuation
    const lastCPU = cpuData[cpuData.length - 1];
    let newCPU = lastCPU + (Math.random() * 12 - 6);
    newCPU = Math.max(5, Math.min(95, newCPU));

    const lastRAM = ramData[ramData.length - 1];
    let newRAM = lastRAM + (Math.random() * 4 - 2);
    newRAM = Math.max(30, Math.min(85, newRAM));

    cpuData.push(newCPU);
    cpuData.shift();

    ramData.push(newRAM);
    ramData.shift();

    // Update text labels
    document.getElementById('val-cpu').innerText = newCPU.toFixed(1) + '%';
    document.getElementById('bar-cpu').style.width = newCPU + '%';

    document.getElementById('val-ram').innerText = ((newRAM / 100) * 16).toFixed(1) + ' GB / 16 GB';
    document.getElementById('bar-ram').style.width = newRAM + '%';

    document.getElementById('val-load').innerText = 
      (newCPU * 0.02).toFixed(2) + ' / ' + (newCPU * 0.018).toFixed(2) + ' / ' + (newCPU * 0.015).toFixed(2);

    document.getElementById('val-rx').innerText = (Math.random() * 15 + 5).toFixed(1) + ' MB/s';
    document.getElementById('val-tx').innerText = (Math.random() * 4 + 1).toFixed(1) + ' MB/s';

    drawChart(ctx, canvas, cpuData, ramData);
  }

  setInterval(tick, 1500);
  tick();
}

function drawChart(ctx, canvas, cpu, ram) {
  const w = canvas.width = canvas.parentElement.offsetWidth;
  const h = canvas.height = 180;

  ctx.clearRect(0, 0, w, h);

  // Draw Grid Lines
  ctx.strokeStyle = '#262626';
  ctx.lineWidth = 1;
  for (let i = 0; i < 4; i++) {
    const y = (h / 4) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }

  // Draw CPU Line (Pure White)
  drawSeries(ctx, w, h, cpu, '#ffffff', 'rgba(255, 255, 255, 0.12)');

  // Draw RAM Line (Silver / Dim White)
  drawSeries(ctx, w, h, ram, '#a3a3a3', 'rgba(163, 163, 163, 0.08)');
}

function drawSeries(ctx, w, h, data, strokeColor, fillColor) {
  const step = w / (data.length - 1);

  ctx.beginPath();
  ctx.moveTo(0, h - (data[0] / 100) * h);

  for (let i = 1; i < data.length; i++) {
    const x = i * step;
    const y = h - (data[i] / 100) * h;
    ctx.lineTo(x, y);
  }

  // Stroke
  ctx.strokeStyle = strokeColor;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Fill gradient
  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = fillColor;
  ctx.fill();
}
