document.addEventListener('DOMContentLoaded', () => {
  initCopyButton();
  initLiveDemo();
  initDrawer();
});

// ─── Copy Button ────────────────────────────────────────────────────────────

function initCopyButton() {
  const copyBtn = document.getElementById('copyBtn');
  const toast   = document.getElementById('toast');
  const cmd = 'curl -sSL https://raw.githubusercontent.com/Woffluon/Statix/main/deploy/install.sh | sudo bash';

  if (!copyBtn) return;

  copyBtn.addEventListener('click', () => {
    navigator.clipboard.writeText(cmd).then(() => {
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

// ─── Animated Counter ───────────────────────────────────────────────────────

function animateValue(el, from, to, duration, suffix, flashClass) {
  if (!el) return;
  const start = performance.now();
  const diff  = to - from;
  function step(now) {
    const t = Math.min((now - start) / duration, 1);
    const ease = 1 - Math.pow(1 - t, 3); // cubic ease-out
    el.textContent = (from + diff * ease).toFixed(1) + suffix;
    if (t < 1) requestAnimationFrame(step);
  }
  // Flash colour
  if (flashClass) {
    el.classList.remove('flash-cpu', 'flash-ram');
    void el.offsetWidth; // reflow to restart animation
    el.classList.add(flashClass);
    setTimeout(() => el.classList.remove(flashClass), 350);
  }
  requestAnimationFrame(step);
}

// ─── Simulation State ───────────────────────────────────────────────────────

const SIM_POINTS = 40;
let cpuData = Array(SIM_POINTS).fill(18);
let ramData = Array(SIM_POINTS).fill(42);

// Per-core simulation (4 cores)
let coreData = [15, 22, 10, 28];

// Network history for sparklines (last 20 values)
let rxHistory = Array(20).fill(12);
let txHistory = Array(20).fill(2);

let prevCPU = 18;
let prevRAM = 42;

// ─── Live Demo ──────────────────────────────────────────────────────────────

function initLiveDemo() {
  const canvas = document.getElementById('demoCanvas');
  if (!canvas) return;

  const ctx = canvas.getContext('2d');

  // Tab switching
  document.querySelectorAll('.tab-btn').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');
      const view = tab.getAttribute('data-view');
      document.querySelectorAll('.demo-view').forEach(v => v.style.display = 'none');
      document.getElementById('view-' + view).style.display = 'block';
    });
  });

  // Process rows → drawer
  document.querySelectorAll('.demo-table tr.clickable').forEach(row => {
    row.addEventListener('click', () => {
      openDrawer('proc', {
        pid:  row.dataset.pid,
        name: row.dataset.name,
      });
    });
  });

  // Stat boxes → drawer
  document.querySelectorAll('.stat-box[data-metric]').forEach(box => {
    box.addEventListener('click', () => openDrawer(box.dataset.metric));
  });

  setInterval(tick, 1500);
  tick();

  function tick() {
    // CPU fluctuation
    let newCPU = cpuData[cpuData.length - 1] + (Math.random() * 14 - 7);
    newCPU = Math.max(4, Math.min(94, newCPU));
    cpuData.push(newCPU); cpuData.shift();

    // RAM fluctuation
    let newRAM = ramData[ramData.length - 1] + (Math.random() * 3 - 1.5);
    newRAM = Math.max(28, Math.min(88, newRAM));
    ramData.push(newRAM); ramData.shift();

    // Per-core
    coreData = coreData.map(v => {
      let n = v + (Math.random() * 18 - 9);
      return Math.max(2, Math.min(98, n));
    });

    // Network
    let rx = 8 + Math.random() * 12;
    let tx = 0.5 + Math.random() * 4;
    rxHistory.push(rx); rxHistory.shift();
    txHistory.push(tx); txHistory.shift();

    // Update stat boxes with counter animation
    const cpuEl = document.getElementById('val-cpu');
    animateValue(cpuEl, prevCPU, newCPU, 400, '%', 'flash-cpu');
    document.getElementById('bar-cpu').style.width = newCPU + '%';

    const ramEl = document.getElementById('val-ram');
    const ramGB = ((newRAM / 100) * 16).toFixed(1);
    if (ramEl) {
      ramEl.textContent = ramGB + ' GB / 16 GB';
      ramEl.classList.remove('flash-ram');
      void ramEl.offsetWidth;
      ramEl.classList.add('flash-ram');
      setTimeout(() => ramEl.classList.remove('flash-ram'), 350);
    }
    document.getElementById('bar-ram').style.width = newRAM + '%';

    document.getElementById('val-load').textContent =
      (newCPU * 0.02).toFixed(2) + ' / ' +
      (newCPU * 0.018).toFixed(2) + ' / ' +
      (newCPU * 0.015).toFixed(2);

    document.getElementById('val-rx').textContent = rx.toFixed(1) + ' MB/s';
    document.getElementById('val-tx').textContent = tx.toFixed(1) + ' MB/s';

    prevCPU = newCPU;
    prevRAM = newRAM;

    drawChart(ctx, canvas, cpuData, ramData);

    // Update drawer if open
    refreshDrawer();
  }
}

// ─── Chart ──────────────────────────────────────────────────────────────────

function drawChart(ctx, canvas, cpu, ram) {
  const w = canvas.width  = canvas.parentElement.offsetWidth;
  const h = canvas.height = 180;

  ctx.clearRect(0, 0, w, h);

  // Grid lines
  ctx.strokeStyle = '#1d1d1d';
  ctx.lineWidth = 1;
  for (let i = 1; i < 4; i++) {
    const y = (h / 4) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }

  drawSeries(ctx, w, h, ram, '#10b981', 'rgba(16,185,129,0.10)');
  drawSeries(ctx, w, h, cpu, '#f59e0b', 'rgba(245,158,11,0.13)');
}

function drawSeries(ctx, w, h, data, strokeColor, fillColor) {
  const step   = w / (data.length - 1);
  const pad    = 8;
  const usable = h - pad * 2;

  ctx.beginPath();
  ctx.moveTo(0, pad + usable - (data[0] / 100) * usable);

  for (let i = 1; i < data.length; i++) {
    const x0 = (i - 1) * step;
    const y0 = pad + usable - (data[i - 1] / 100) * usable;
    const x1 = i * step;
    const y1 = pad + usable - (data[i] / 100) * usable;
    const cpx = (x0 + x1) / 2;
    ctx.bezierCurveTo(cpx, y0, cpx, y1, x1, y1);
  }

  ctx.strokeStyle = strokeColor;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Fill
  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();

  const grad = ctx.createLinearGradient(0, 0, 0, h);
  grad.addColorStop(0, fillColor);
  grad.addColorStop(1, 'transparent');
  ctx.fillStyle = grad;
  ctx.fill();
}

// ─── Sparkline (mini canvas) ────────────────────────────────────────────────

function drawSparkline(canvas, data, color) {
  if (!canvas) return;
  const w = canvas.width  = canvas.offsetWidth || 300;
  const h = canvas.height = 60;
  const ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, w, h);

  const max  = Math.max(...data, 1);
  const step = w / (data.length - 1);
  const pad  = 4;

  ctx.beginPath();
  ctx.moveTo(0, pad + (h - pad * 2) - (data[0] / max) * (h - pad * 2));
  for (let i = 1; i < data.length; i++) {
    const x = i * step;
    const y = pad + (h - pad * 2) - (data[i] / max) * (h - pad * 2);
    ctx.lineTo(x, y);
  }
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = color.replace(')', ', 0.10)').replace('rgb', 'rgba');
  ctx.fill();
}

// ─── Drawer ─────────────────────────────────────────────────────────────────

let currentDrawerMetric = null;
let currentDrawerExtra  = null;

function initDrawer() {
  const overlay = document.getElementById('drawer-overlay');
  const closeBtn = document.getElementById('drawer-close');

  overlay.addEventListener('click', e => {
    if (e.target === overlay) closeDrawer();
  });
  closeBtn.addEventListener('click', closeDrawer);

  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') closeDrawer();
  });
}

function openDrawer(metric, extra = null) {
  currentDrawerMetric = metric;
  currentDrawerExtra  = extra;

  renderDrawerContent(metric, extra);

  const overlay = document.getElementById('drawer-overlay');
  overlay.classList.add('open');
  document.body.style.overflow = 'hidden';
}

function closeDrawer() {
  document.getElementById('drawer-overlay').classList.remove('open');
  document.body.style.overflow = '';
  currentDrawerMetric = null;
  currentDrawerExtra  = null;
}

function refreshDrawer() {
  if (!currentDrawerMetric) return;
  renderDrawerContent(currentDrawerMetric, currentDrawerExtra);
}

function renderDrawerContent(metric, extra) {
  const title = document.getElementById('drawer-title');
  const body  = document.getElementById('drawer-body');

  switch (metric) {
    case 'cpu':  renderCPUDrawer(title, body);  break;
    case 'ram':  renderRAMDrawer(title, body);  break;
    case 'net':  renderNetDrawer(title, body);  break;
    case 'disk': renderDiskDrawer(title, body); break;
    case 'proc': renderProcDrawer(title, body, extra); break;
    default: break;
  }
}

function renderCPUDrawer(title, body) {
  title.textContent = 'CPU — Per-Core Utilization';
  const currentCPU = cpuData[cpuData.length - 1];

  body.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Total CPU</div>
      <div style="font-size: 2rem; font-weight: 700; font-family: var(--font-mono); color: var(--accent-cpu); margin-bottom: 0.5rem;">
        ${currentCPU.toFixed(1)}%
      </div>
      <div class="progress-track"><div class="progress-fill progress-fill--cpu" style="width: ${currentCPU}%;"></div></div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">Per-Core Breakdown</div>
      ${coreData.map((pct, i) => `
        <div class="core-bar-row">
          <span class="core-bar-label">Core ${i}</span>
          <div class="core-bar-track"><div class="core-bar-fill" style="width: ${pct.toFixed(1)}%;"></div></div>
          <span class="core-bar-pct">${pct.toFixed(1)}%</span>
        </div>
      `).join('')}
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">60-Second Trend</div>
      <canvas class="sparkline-canvas" id="sparkline-cpu"></canvas>
    </div>
  `;

  // Draw sparkline with last 20 cpu points
  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('sparkline-cpu'), cpuData.slice(-20), '#f59e0b');
  });
}

function renderRAMDrawer(title, body) {
  title.textContent = 'RAM — Memory Breakdown';
  const ramPct  = ramData[ramData.length - 1];
  const ramUsed = (ramPct / 100) * 16;
  const buffers = +(ramUsed * 0.12).toFixed(2);
  const cached  = +(ramUsed * 0.28).toFixed(2);
  const used    = +(ramUsed - buffers - cached).toFixed(2);
  const free    = +(16 - ramUsed).toFixed(2);

  body.innerHTML = `
    <div class="drawer-section">
      <div style="font-size: 1.8rem; font-weight: 700; font-family: var(--font-mono); color: var(--accent-ram); margin-bottom: 0.5rem;">
        ${ramUsed.toFixed(1)} GB <span style="font-size: 1rem; color: var(--text-dim);">/ 16 GB</span>
      </div>
      <div class="progress-track"><div class="progress-fill progress-fill--ram" style="width: ${ramPct}%;"></div></div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">Memory Segments</div>
      <div class="mem-breakdown">
        <div class="mem-row">
          <span class="mem-row-label">Used (applications)</span>
          <span class="mem-row-value">${used} GB</span>
        </div>
        <div class="mem-row">
          <span class="mem-row-label">Buffers</span>
          <span class="mem-row-value">${buffers} GB</span>
        </div>
        <div class="mem-row">
          <span class="mem-row-label">Cached</span>
          <span class="mem-row-value">${cached} GB</span>
        </div>
        <div class="mem-row" style="border-color: var(--accent-ram); border-opacity: 0.3;">
          <span class="mem-row-label">Free</span>
          <span class="mem-row-value" style="color: var(--accent-ram);">${free} GB</span>
        </div>
      </div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">60-Second Trend</div>
      <canvas class="sparkline-canvas" id="sparkline-ram"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('sparkline-ram'), ramData.slice(-20), '#10b981');
  });
}

function renderNetDrawer(title, body) {
  title.textContent = 'Network — Interface Throughput';

  body.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">eth0</div>
      <div class="net-row">
        <span class="net-iface">eth0</span>
        <div class="net-stat">
          <span class="net-stat-label">RX</span>
          <span class="net-stat-value">${rxHistory[rxHistory.length - 1].toFixed(1)} MB/s</span>
        </div>
        <div class="net-stat">
          <span class="net-stat-label">TX</span>
          <span class="net-stat-value">${txHistory[txHistory.length - 1].toFixed(1)} MB/s</span>
        </div>
      </div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">RX Trend (last 20 ticks)</div>
      <canvas class="sparkline-canvas" id="sparkline-rx"></canvas>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">TX Trend (last 20 ticks)</div>
      <canvas class="sparkline-canvas" id="sparkline-tx"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('sparkline-rx'), rxHistory, '#38bdf8');
    drawSparkline(document.getElementById('sparkline-tx'), txHistory, '#a78bfa');
  });
}

function renderDiskDrawer(title, body) {
  title.textContent = 'Disk — Storage & I/O';

  body.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Partitions</div>
      <div style="font-family: var(--font-mono); font-size: 0.83rem;">
        <div class="disk-row" style="display:flex; justify-content:space-between; padding: 0.5rem 0; border-bottom: 1px solid var(--border-color);">
          <span style="color: var(--text-dim); width: 70px;">/dev/sda1</span>
          <span style="color: var(--accent-cpu);">38.2%</span>
          <span>76.4 GB / 200 GB</span>
        </div>
        <div class="disk-row" style="display:flex; justify-content:space-between; padding: 0.5rem 0;">
          <span style="color: var(--text-dim); width: 70px;">/dev/sdb1</span>
          <span style="color: var(--accent-ram);">12.5%</span>
          <span>25 GB / 200 GB</span>
        </div>
      </div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">Read/Write I/O (sda)</div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; margin-bottom: 0.75rem;">
        <div class="mem-row">
          <span class="mem-row-label">Read</span>
          <span class="mem-row-value" style="color: var(--accent-net);">${(Math.random() * 2).toFixed(1)} KB/s</span>
        </div>
        <div class="mem-row">
          <span class="mem-row-label">Write</span>
          <span class="mem-row-value" style="color: var(--accent-disk);">${(Math.random() * 1).toFixed(1)} KB/s</span>
        </div>
      </div>
      <canvas class="sparkline-canvas" id="sparkline-disk"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    const fakeIO = Array(20).fill(0).map(() => Math.random() * 3);
    drawSparkline(document.getElementById('sparkline-disk'), fakeIO, '#a78bfa');
  });
}

function renderProcDrawer(title, body, extra) {
  const name = extra?.name || 'process';
  const pid  = extra?.pid  || '???';
  title.textContent = `Process — ${name} (PID ${pid})`;

  const fakeCPU = (Math.random() * 5).toFixed(2);
  const fakeRSS = (Math.random() * 200 + 10).toFixed(1);
  const fakeFDs = Math.floor(Math.random() * 30 + 5);

  body.innerHTML = `
    <div class="drawer-section">
      <div class="mem-breakdown">
        <div class="mem-row"><span class="mem-row-label">PID</span><span class="mem-row-value">${pid}</span></div>
        <div class="mem-row"><span class="mem-row-label">Name</span><span class="mem-row-value" style="color: var(--accent-proc);">${name}</span></div>
        <div class="mem-row"><span class="mem-row-label">State</span><span class="mem-row-value">S (sleeping)</span></div>
        <div class="mem-row"><span class="mem-row-label">CPU %</span><span class="mem-row-value" style="color: var(--accent-cpu);">${fakeCPU}%</span></div>
        <div class="mem-row"><span class="mem-row-label">RSS</span><span class="mem-row-value" style="color: var(--accent-ram);">${fakeRSS} MB</span></div>
        <div class="mem-row"><span class="mem-row-label">Open FDs</span><span class="mem-row-value">${fakeFDs}</span></div>
      </div>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title">CPU History</div>
      <canvas class="sparkline-canvas" id="sparkline-proc"></canvas>
    </div>
    <div class="drawer-section">
      <div class="drawer-section-title" style="color: var(--text-dim); font-style: italic;">
        Process tree requires ppid field — available in live dashboard
      </div>
    </div>
  `;

  requestAnimationFrame(() => {
    const history = Array(20).fill(0).map(() => Math.random() * parseFloat(fakeCPU) * 2);
    drawSparkline(document.getElementById('sparkline-proc'), history, '#fb7185');
  });
}
