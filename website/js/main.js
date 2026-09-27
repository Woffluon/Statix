/**
 * Statix Live Telemetry Simulator & Interactions
 * High-performance, zero-dependency, DPI-aware Vanilla JS engine.
 */

document.addEventListener('DOMContentLoaded', () => {
  initClipboardActions();
  initMobileMenu();
  initQuickrunTabs();
  initLiveDemo();
  initDrawer();
});

// ─── Clipboard & Copy Handlers ──────────────────────────────────────────────
function initClipboardActions() {
  const copyBtn = document.getElementById('copyBtn');
  const installCmd = 'curl -sSL https://raw.githubusercontent.com/Woffluon/Statix/main/deploy/install.sh | sudo bash';

  if (copyBtn) {
    copyBtn.addEventListener('click', () => {
      copyToClipboard(installCmd, copyBtn, 'Copied');
    });
  }

  // Keyboard shortcut: Cmd+K or Ctrl+K triggers copy
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      copyToClipboard(installCmd, copyBtn, 'Copied');
    }
  });

  // Checksum copy buttons
  document.querySelectorAll('.checksum-copy-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const hash = btn.getAttribute('data-copy');
      if (hash) {
        copyToClipboard(hash, btn, 'Copied');
      }
    });
  });
}

function copyToClipboard(text, targetBtn, feedbackText) {
  navigator.clipboard.writeText(text).then(() => {
    showToast(feedbackText ? feedbackText + ' to clipboard' : 'Copied to clipboard');
    if (targetBtn) {
      const originalHTML = targetBtn.innerHTML;
      targetBtn.innerHTML = `
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
        <span class="copy-text">${feedbackText || 'Copied'}</span>
      `;
      targetBtn.style.borderColor = 'var(--accent-ram)';
      setTimeout(() => {
        targetBtn.innerHTML = originalHTML;
        targetBtn.style.borderColor = '';
      }, 2000);
    }
  }).catch(() => {
    showToast('Failed to copy text to clipboard');
  });
}

function showToast(message) {
  const toast = document.getElementById('toast');
  const msgEl = document.getElementById('toastMessage');
  if (!toast) return;

  if (msgEl) msgEl.textContent = message;
  toast.classList.add('show');
  clearTimeout(window.__toastTimeout);
  window.__toastTimeout = setTimeout(() => {
    toast.classList.remove('show');
  }, 2400);
}

// ─── Mobile Navigation Drawer ───────────────────────────────────────────────
function initMobileMenu() {
  const toggleBtn = document.getElementById('mobileMenuToggle');
  const drawer    = document.getElementById('mobileNavDrawer');
  const backdrop  = document.getElementById('mobileNavBackdrop');
  const closeBtn  = document.getElementById('mobileNavClose');

  if (!toggleBtn || !drawer || !backdrop) return;

  function openMenu() {
    drawer.classList.add('open');
    backdrop.classList.add('open');
    drawer.setAttribute('aria-hidden', 'false');
    toggleBtn.setAttribute('aria-expanded', 'true');
    document.body.style.overflow = 'hidden';
  }

  function closeMenu() {
    drawer.classList.remove('open');
    backdrop.classList.remove('open');
    drawer.setAttribute('aria-hidden', 'true');
    toggleBtn.setAttribute('aria-expanded', 'false');
    document.body.style.overflow = '';
  }

  toggleBtn.addEventListener('click', openMenu);
  if (closeBtn) closeBtn.addEventListener('click', closeMenu);
  backdrop.addEventListener('click', closeMenu);

  document.querySelectorAll('.mobile-nav-link').forEach((link) => {
    link.addEventListener('click', closeMenu);
  });
}

// ─── Quickrun Code Tabs ─────────────────────────────────────────────────────
function initQuickrunTabs() {
  const tabBtns = document.querySelectorAll('.qr-tab-btn');
  tabBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      tabBtns.forEach((b) => b.classList.remove('active'));
      btn.classList.add('active');

      const targetId = 'tab-' + btn.dataset.tab;
      document.querySelectorAll('.qr-content').forEach((panel) => {
        panel.style.display = panel.id === targetId ? 'block' : 'none';
      });
    });
  });
}

// ─── Homelab Process Metadata Catalog ──────────────────────────────────────
const HOMELAB_SERVICES = {
  '1042': {
    name: 'statix',
    cmdline: '/usr/local/bin/statix --addr 0.0.0.0:8080',
    user: 'statix',
    uid: '1001',
    gid: '1001',
    state: 'S (Interruptible Sleep)',
    cpu: '0.1',
    rss: '18.2 MB',
    vmsize: '28.4 MB',
    threads: 6,
    fds: 18,
    statusText: `Name:\tstatix
Umask:\t0022
State:\tS (sleeping)
Tgid:\t1042
Ngid:\t0
Pid:\t1042
PPid:\t1
TracerPid:\t0
Uid:\t1001\t1001\t1001\t1001
Gid:\t1001\t1001\t1001\t1001
FDSize:\t64
VmPeak:\t   32412 kB
VmSize:\t   28410 kB
VmLck:\t       0 kB
VmHWM:\t   18420 kB
VmRSS:\t   18240 kB
Threads:\t6`
  },
  '621': {
    name: 'caddy',
    cmdline: '/usr/bin/caddy run --config /etc/caddy/Caddyfile',
    user: 'caddy',
    uid: '998',
    gid: '998',
    state: 'S (Interruptible Sleep)',
    cpu: '0.4',
    rss: '32.4 MB',
    vmsize: '74.8 MB',
    threads: 10,
    fds: 24,
    statusText: `Name:\tcaddy
Umask:\t0022
State:\tS (sleeping)
Tgid:\t621
Ngid:\t0
Pid:\t621
PPid:\t1
TracerPid:\t0
Uid:\t998\t998\t998\t998
Gid:\t998\t998\t998\t998
FDSize:\t128
VmPeak:\t   78210 kB
VmSize:\t   74800 kB
VmLck:\t       0 kB
VmHWM:\t   34120 kB
VmRSS:\t   32400 kB
Threads:\t10`
  },
  '789': {
    name: 'tailscaled',
    cmdline: '/usr/sbin/tailscaled --state=/var/lib/tailscale/tailscaled.state',
    user: 'tailscale',
    uid: '994',
    gid: '994',
    state: 'S (Interruptible Sleep)',
    cpu: '0.3',
    rss: '28.6 MB',
    vmsize: '52.1 MB',
    threads: 8,
    fds: 20,
    statusText: `Name:\ttailscaled
Umask:\t0022
State:\tS (sleeping)
Tgid:\t789
Ngid:\t0
Pid:\t789
PPid:\t1
TracerPid:\t0
Uid:\t994\t994\t994\t994
Gid:\t994\t994\t994\t994
FDSize:\t64
VmPeak:\t   56200 kB
VmSize:\t   52100 kB
VmLck:\t       0 kB
VmHWM:\t   29800 kB
VmRSS:\t   28600 kB
Threads:\t8`
  },
  '892': {
    name: 'docker',
    cmdline: '/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock',
    user: 'root',
    uid: '0',
    gid: '0',
    state: 'S (Interruptible Sleep)',
    cpu: '1.2',
    rss: '84.5 MB',
    vmsize: '1420.2 MB',
    threads: 28,
    fds: 64,
    statusText: `Name:\tdockerd
Umask:\t0022
State:\tS (sleeping)
Tgid:\t892
Ngid:\t0
Pid:\t892
PPid:\t1
TracerPid:\t0
Uid:\t0\t0\t0\t0
Gid:\t0\t0\t0\t0
FDSize:\t256
VmPeak:\t 1450200 kB
VmSize:\t 1420200 kB
VmLck:\t       0 kB
VmHWM:\t   88200 kB
VmRSS:\t   84500 kB
Threads:\t28`
  },
  '1105': {
    name: 'pihole-FTL',
    cmdline: '/usr/bin/pihole-FTL -f',
    user: 'pihole',
    uid: '996',
    gid: '996',
    state: 'S (Interruptible Sleep)',
    cpu: '0.5',
    rss: '24.1 MB',
    vmsize: '48.9 MB',
    threads: 5,
    fds: 16,
    statusText: `Name:\tpihole-FTL
Umask:\t0022
State:\tS (sleeping)
Tgid:\t1105
Ngid:\t0
Pid:\t1105
PPid:\t1
TracerPid:\t0
Uid:\t996\t996\t996\t996
Gid:\t996\t996\t996\t996
FDSize:\t64
VmPeak:\t   52100 kB
VmSize:\t   48900 kB
VmLck:\t       0 kB
VmHWM:\t   26400 kB
VmRSS:\t   24100 kB
Threads:\t5`
  },
  '1450': {
    name: 'jellyfin',
    cmdline: '/usr/bin/jellyfin --datadir /var/lib/jellyfin',
    user: 'jellyfin',
    uid: '992',
    gid: '992',
    state: 'S (Interruptible Sleep)',
    cpu: '3.8',
    rss: '342.0 MB',
    vmsize: '2840.0 MB',
    threads: 36,
    fds: 92,
    statusText: `Name:\tjellyfin
Umask:\t0022
State:\tS (sleeping)
Tgid:\t1450
Ngid:\t0
Pid:\t1450
PPid:\t1
TracerPid:\t0
Uid:\t992\t992\t992\t992
Gid:\t992\t992\t992\t992
FDSize:\t256
VmPeak:\t 2910000 kB
VmSize:\t 2840000 kB
VmLck:\t       0 kB
VmHWM:\t  362000 kB
VmRSS:\t  342000 kB
Threads:\t36`
  },
  '1680': {
    name: 'postgres',
    cmdline: 'postgres: 16/main: checkpointer writer process',
    user: 'postgres',
    uid: '999',
    gid: '999',
    state: 'S (Interruptible Sleep)',
    cpu: '1.8',
    rss: '186.4 MB',
    vmsize: '382.5 MB',
    threads: 1,
    fds: 42,
    statusText: `Name:\tpostgres
Umask:\t0077
State:\tS (sleeping)
Tgid:\t1680
Ngid:\t0
Pid:\t1680
PPid:\t1
TracerPid:\t0
Uid:\t999\t999\t999\t999
Gid:\t999\t999\t999\t999
FDSize:\t128
VmPeak:\t  394100 kB
VmSize:\t  382500 kB
VmLck:\t       0 kB
VmHWM:\t  192400 kB
VmRSS:\t  186400 kB
Threads:\t1`
  },
  '2041': {
    name: 'vaultwarden',
    cmdline: '/vaultwarden',
    user: 'vaultwarden',
    uid: '1002',
    gid: '1002',
    state: 'S (Interruptible Sleep)',
    cpu: '0.2',
    rss: '46.8 MB',
    vmsize: '98.2 MB',
    threads: 12,
    fds: 28,
    statusText: `Name:\tvaultwarden
Umask:\t0022
State:\tS (sleeping)
Tgid:\t2041
Ngid:\t0
Pid:\t2041
PPid:\t1
TracerPid:\t0
Uid:\t1002\t1002\t1002\t1002
Gid:\t1002\t1002\t1002\t1002
FDSize:\t64
VmPeak:\t  104200 kB
VmSize:\t   98200 kB
VmLck:\t       0 kB
VmHWM:\t   49200 kB
VmRSS:\t   46800 kB
Threads:\t12`
  }
};

// ─── Telemetry State & Simulation Ring Buffer ──────────────────────────────
const SIM_POINTS = 50;
let cpuHistory = Array(SIM_POINTS).fill(18.5);
let ramHistory = Array(SIM_POINTS).fill(26.2);

// 8 Cores array
let coreUtilization = [14.2, 22.8, 9.4, 31.0, 18.2, 12.6, 24.1, 16.7];

// Network sparkline buffers (25 ticks)
let rxBuffer = Array(25).fill(14.2);
let txBuffer = Array(25).fill(2.6);

let isSimulationPaused = false;
let simulationInterval = null;

// ─── Live Demo Telemetry Controller ─────────────────────────────────────────
function initLiveDemo() {
  const canvas = document.getElementById('demoCanvas');
  if (!canvas) return;

  // View tabs switcher
  const tabBtns = document.querySelectorAll('.demo-tabs .tab-btn');
  tabBtns.forEach((tab) => {
    tab.addEventListener('click', () => {
      tabBtns.forEach((t) => {
        t.classList.remove('active');
        t.setAttribute('aria-selected', 'false');
      });
      tab.classList.add('active');
      tab.setAttribute('aria-selected', 'true');

      const viewId = 'view-' + tab.dataset.view;
      document.querySelectorAll('.demo-view').forEach((view) => {
        view.style.display = view.id === viewId ? 'block' : 'none';
      });

      if (tab.dataset.view === 'overview') {
        renderCanvasChart();
      }
    });
  });

  // Pause / Resume Stream Toggle
  const pauseBtn = document.getElementById('telemetryPauseBtn');
  const pauseIcon = document.getElementById('pauseBtnIcon');
  const pauseText = document.getElementById('pauseBtnText');
  if (pauseBtn) {
    pauseBtn.addEventListener('click', () => {
      isSimulationPaused = !isSimulationPaused;
      if (isSimulationPaused) {
        pauseIcon.textContent = '▶';
        pauseText.textContent = 'Resume';
        pauseBtn.style.color = 'var(--accent-cpu)';
        pauseBtn.style.borderColor = 'var(--accent-cpu)';
      } else {
        pauseIcon.textContent = '⏸';
        pauseText.textContent = 'Pause';
        pauseBtn.style.color = '';
        pauseBtn.style.borderColor = '';
      }
    });
  }

  // Process rows click -> open drawer
  document.querySelectorAll('#processTableBody tr').forEach((row) => {
    row.addEventListener('click', () => {
      openDrawer('proc', {
        pid: row.dataset.pid,
        name: row.dataset.name,
        user: row.dataset.user,
        cpu: row.dataset.cpu,
        rss: row.dataset.rss,
      });
    });
  });

  // Stat boxes click -> open metric drawer
  document.querySelectorAll('.stat-box[data-metric]').forEach((box) => {
    box.addEventListener('click', () => {
      openDrawer(box.dataset.metric);
    });
    box.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        openDrawer(box.dataset.metric);
      }
    });
  });

  // Window resize observer for responsive canvas DPI
  let resizeTimeout;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
      renderCanvasChart();
    }, 100);
  });

  // Render initial cores mini grid
  updateCoresMiniGrid();

  // Initial draw & timer
  renderCanvasChart();
  simulationInterval = setInterval(telemetryTick, 1200);
}

// ─── Periodic Telemetry Tick ────────────────────────────────────────────────
function telemetryTick() {
  if (isSimulationPaused) return;

  // CPU Random Walk with mean-reversion around 19%
  const lastCPU = cpuHistory[cpuHistory.length - 1];
  const cpuDelta = (Math.random() * 10 - 4.8) - (lastCPU - 19) * 0.15;
  const newCPU = Math.max(5.0, Math.min(84.0, lastCPU + cpuDelta));
  cpuHistory.push(newCPU);
  cpuHistory.shift();

  // RAM Random Walk (slow drift around 26.2% of 32 GB)
  const lastRAM = ramHistory[ramHistory.length - 1];
  const ramDelta = (Math.random() * 1.8 - 0.88);
  const newRAM = Math.max(22.0, Math.min(48.0, lastRAM + ramDelta));
  ramHistory.push(newRAM);
  ramHistory.shift();

  // 8-Core Fluctuations
  coreUtilization = coreUtilization.map((val) => {
    const delta = (Math.random() * 14 - 7) - (val - newCPU) * 0.2;
    return Math.max(2.0, Math.min(94.0, val + delta));
  });

  // Network I/O Rates
  const newRX = Math.max(2.4, 12.0 + Math.sin(Date.now() / 3200) * 5 + (Math.random() * 3.5));
  const newTX = Math.max(0.6, 2.2 + Math.cos(Date.now() / 4200) * 1.4 + (Math.random() * 1.2));
  rxBuffer.push(newRX);
  rxBuffer.shift();
  txBuffer.push(newTX);
  txBuffer.shift();

  // Disk I/O Rates
  const diskRead = (Math.random() * 3.2 + 0.5).toFixed(1);
  const diskWrite = Math.floor(Math.random() * 450 + 200);

  // Update UI Elements
  const cpuEl = document.getElementById('val-cpu');
  if (cpuEl) cpuEl.textContent = newCPU.toFixed(1) + '%';
  const barCpu = document.getElementById('bar-cpu');
  if (barCpu) barCpu.style.width = newCPU.toFixed(1) + '%';

  const ramEl = document.getElementById('val-ram');
  const ramGB = ((newRAM / 100) * 32).toFixed(1);
  if (ramEl) ramEl.textContent = `${ramGB} GB / 32 GB`;
  const barRam = document.getElementById('bar-ram');
  if (barRam) barRam.style.width = newRAM.toFixed(1) + '%';

  // Load Average calculation
  const l1  = (newCPU * 0.022 + 0.1).toFixed(2);
  const l5  = (newCPU * 0.019 + 0.12).toFixed(2);
  const l15 = (newCPU * 0.016 + 0.15).toFixed(2);
  const loadEl = document.getElementById('val-load');
  if (loadEl) loadEl.textContent = `${l1} / ${l5} / ${l15}`;
  const load1m = document.getElementById('load-1m');
  const load5m = document.getElementById('load-5m');
  const load15m = document.getElementById('load-15m');
  if (load1m) load1m.textContent = l1;
  if (load5m) load5m.textContent = l5;
  if (load15m) load15m.textContent = l15;

  // Network DOM
  const rxEl = document.getElementById('val-rx');
  const txEl = document.getElementById('val-tx');
  if (rxEl) rxEl.textContent = newRX.toFixed(1) + ' MB/s';
  if (txEl) txEl.textContent = newTX.toFixed(1) + ' MB/s';

  // Disk DOM
  const diskREl = document.getElementById('val-diskr');
  const diskWEl = document.getElementById('val-diskw');
  if (diskREl) diskREl.textContent = diskRead + ' MB/s';
  if (diskWEl) diskWEl.textContent = diskWrite + ' KB/s';

  // Cores Mini Grid
  updateCoresMiniGrid();

  // Render Canvas
  renderCanvasChart();

  // Refresh active drawer if open
  refreshActiveDrawer();
}

function updateCoresMiniGrid() {
  const container = document.getElementById('coreBarsMiniGrid');
  if (!container) return;

  container.innerHTML = coreUtilization.map((pct, idx) => `
    <div class="core-cell">
      <div class="core-cell-header">
        <span class="core-cell-name">C${idx}</span>
        <span class="core-cell-val">${pct.toFixed(0)}%</span>
      </div>
      <div class="core-mini-track">
        <div class="core-mini-fill" style="width: ${pct.toFixed(1)}%;"></div>
      </div>
    </div>
  `).join('');
}

// ─── High DPI Canvas Chart Rendering ────────────────────────────────────────
function renderCanvasChart() {
  const canvas = document.getElementById('demoCanvas');
  if (!canvas) return;

  const rect = canvas.getBoundingClientRect();
  if (rect.width === 0) return;

  const dpr = window.devicePixelRatio || 1;
  const w = rect.width;
  const h = rect.height || 190;

  // Scale canvas resolution to physical pixels for crisp edges
  canvas.width = Math.floor(w * dpr);
  canvas.height = Math.floor(h * dpr);

  const ctx = canvas.getContext('2d');
  ctx.save();
  ctx.scale(dpr, dpr);

  ctx.clearRect(0, 0, w, h);

  // Background Grid Lines & Scale Markers
  const gridLines = 4;
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
  ctx.lineWidth = 1;
  ctx.font = '10px "JetBrains Mono", monospace';
  ctx.fillStyle = '#94a3b8';

  for (let i = 0; i <= gridLines; i++) {
    const y = Math.floor((h / gridLines) * i);
    const pct = 100 - (i * 25);

    ctx.beginPath();
    ctx.moveTo(35, y === 0 ? 1 : y);
    ctx.lineTo(w, y === 0 ? 1 : y);
    ctx.stroke();

    ctx.fillText(`${pct}%`, 4, y === 0 ? 10 : (y === h ? y - 4 : y + 3));
  }

  // Draw RAM Series (Emerald)
  drawSmoothSeries(ctx, w, h, ramHistory, '#10b981', 'rgba(16, 185, 129, 0.12)');

  // Draw CPU Series (Amber)
  drawSmoothSeries(ctx, w, h, cpuHistory, '#f59e0b', 'rgba(245, 158, 11, 0.15)');

  ctx.restore();
}

function drawSmoothSeries(ctx, w, h, data, strokeColor, fillColor) {
  const leftPad = 35;
  const usableW = w - leftPad;
  const usableH = h - 16;
  const step = usableW / (data.length - 1);

  ctx.beginPath();
  const firstY = 8 + usableH - (data[0] / 100) * usableH;
  ctx.moveTo(leftPad, firstY);

  for (let i = 1; i < data.length; i++) {
    const x0 = leftPad + (i - 1) * step;
    const y0 = 8 + usableH - (data[i - 1] / 100) * usableH;
    const x1 = leftPad + i * step;
    const y1 = 8 + usableH - (data[i] / 100) * usableH;
    const cpx = (x0 + x1) / 2;
    ctx.bezierCurveTo(cpx, y0, cpx, y1, x1, y1);
  }

  ctx.strokeStyle = strokeColor;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Fill gradient
  ctx.lineTo(w, h);
  ctx.lineTo(leftPad, h);
  ctx.closePath();

  const grad = ctx.createLinearGradient(0, 0, 0, h);
  grad.addColorStop(0, fillColor);
  grad.addColorStop(1, 'rgba(0, 0, 0, 0)');
  ctx.fillStyle = grad;
  ctx.fill();

  // Live tip point
  const lastIndex = data.length - 1;
  const lastX = leftPad + lastIndex * step;
  const lastY = 8 + usableH - (data[lastIndex] / 100) * usableH;

  ctx.beginPath();
  ctx.arc(lastX, lastY, 3.5, 0, Math.PI * 2);
  ctx.fillStyle = strokeColor;
  ctx.fill();
}

// ─── Sparklines for Drawers ─────────────────────────────────────────────────
function drawSparkline(canvas, data, color) {
  if (!canvas) return;
  const dpr = window.devicePixelRatio || 1;
  const w = canvas.parentElement.offsetWidth || 340;
  const h = 60;

  canvas.width = Math.floor(w * dpr);
  canvas.height = Math.floor(h * dpr);

  const ctx = canvas.getContext('2d');
  ctx.save();
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);

  const max = Math.max(...data, 1);
  const step = w / (data.length - 1);
  const pad = 4;
  const usableH = h - pad * 2;

  ctx.beginPath();
  ctx.moveTo(0, pad + usableH - (data[0] / max) * usableH);

  for (let i = 1; i < data.length; i++) {
    const x = i * step;
    const y = pad + usableH - (data[i] / max) * usableH;
    ctx.lineTo(x, y);
  }

  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = color.replace(')', ', 0.12)').replace('rgb', 'rgba');
  ctx.fill();

  ctx.restore();
}

// ─── Drawer Modal Controller ────────────────────────────────────────────────
let activeDrawerType = null;
let activeDrawerContext = null;

function initDrawer() {
  const overlay = document.getElementById('drawer-overlay');
  const closeBtn = document.getElementById('drawer-close');

  if (!overlay) return;

  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) closeDrawer();
  });

  if (closeBtn) {
    closeBtn.addEventListener('click', closeDrawer);
  }

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeDrawer();
  });
}

function openDrawer(type, context = null) {
  activeDrawerType = type;
  activeDrawerContext = context;

  renderDrawerBody(type, context);

  const overlay = document.getElementById('drawer-overlay');
  overlay.classList.add('open');
  overlay.setAttribute('aria-hidden', 'false');
  document.body.style.overflow = 'hidden';
}

function closeDrawer() {
  const overlay = document.getElementById('drawer-overlay');
  if (!overlay) return;
  overlay.classList.remove('open');
  overlay.setAttribute('aria-hidden', 'true');
  document.body.style.overflow = '';
  activeDrawerType = null;
  activeDrawerContext = null;
}

function refreshActiveDrawer() {
  if (!activeDrawerType) return;
  renderDrawerBody(activeDrawerType, activeDrawerContext);
}

function renderDrawerBody(type, context) {
  const titleEl = document.getElementById('drawer-title');
  const bodyEl  = document.getElementById('drawer-body');
  if (!titleEl || !bodyEl) return;

  switch (type) {
    case 'cpu':
      renderCpuDrawer(titleEl, bodyEl);
      break;
    case 'ram':
      renderRamDrawer(titleEl, bodyEl);
      break;
    case 'net':
      renderNetDrawer(titleEl, bodyEl);
      break;
    case 'disk':
      renderDiskDrawer(titleEl, bodyEl);
      break;
    case 'proc':
      renderProcDrawer(titleEl, bodyEl, context);
      break;
    default:
      break;
  }
}

function renderCpuDrawer(titleEl, bodyEl) {
  titleEl.textContent = 'CPU: Core Telemetry and Utilization';
  const curCPU = cpuHistory[cpuHistory.length - 1];

  bodyEl.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Total CPU Utilization</div>
      <div style="font-size: 2.2rem; font-weight: 800; font-family: var(--font-mono); color: var(--accent-cpu); margin-bottom: 0.5rem;">
        ${curCPU.toFixed(1)}%
      </div>
      <div class="progress-track"><div class="progress-fill progress-fill--cpu" style="width: ${curCPU.toFixed(1)}%;"></div></div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">8 Logical Cores Breakdown (/proc/stat)</div>
      <div class="drawer-grid">
        ${coreUtilization.map((val, idx) => `
          <div class="drawer-row">
            <span class="drawer-row-lbl">Core ${idx}</span>
            <div style="flex:1; margin: 0 1rem; height: 5px; background: rgba(255,255,255,0.06); border-radius:3px; overflow:hidden;">
              <div style="height:100%; width:${val.toFixed(1)}%; background:var(--accent-cpu);"></div>
            </div>
            <span class="drawer-row-val">${val.toFixed(1)}%</span>
          </div>
        `).join('')}
      </div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">60-Second CPU Activity Trend</div>
      <canvas class="sparkline-canvas" id="spark-cpu"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('spark-cpu'), cpuHistory.slice(-25), '#f59e0b');
  });
}

function renderRamDrawer(titleEl, bodyEl) {
  titleEl.textContent = 'RAM: Memory Distribution (/proc/meminfo)';
  const curRAM = ramHistory[ramHistory.length - 1];
  const totalGB = 32.0;
  const usedGB  = (curRAM / 100) * totalGB;
  const cachedGB = (usedGB * 0.35).toFixed(2);
  const bufferGB = (usedGB * 0.10).toFixed(2);
  const appGB    = (usedGB - parseFloat(cachedGB) - parseFloat(bufferGB)).toFixed(2);
  const freeGB   = (totalGB - usedGB).toFixed(2);

  bodyEl.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Total Allocated Memory</div>
      <div style="font-size: 2.2rem; font-weight: 800; font-family: var(--font-mono); color: var(--accent-ram); margin-bottom: 0.5rem;">
        ${usedGB.toFixed(1)} GB <span style="font-size: 1rem; color: var(--text-dim);">/ 32.0 GB</span>
      </div>
      <div class="progress-track"><div class="progress-fill progress-fill--ram" style="width: ${curRAM.toFixed(1)}%;"></div></div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">Kernel Memory Segments</div>
      <div class="drawer-grid">
        <div class="drawer-row">
          <span class="drawer-row-lbl">Application Memory (Active)</span>
          <span class="drawer-row-val">${appGB} GB</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">VFS Page Cache (Cached)</span>
          <span class="drawer-row-val text-net">${cachedGB} GB</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">I/O Buffers (Buffers)</span>
          <span class="drawer-row-val text-disk">${bufferGB} GB</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">Available Memory (MemFree)</span>
          <span class="drawer-row-val text-ram">${freeGB} GB</span>
        </div>
      </div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">Memory Allocation Trend</div>
      <canvas class="sparkline-canvas" id="spark-ram"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('spark-ram'), ramHistory.slice(-25), '#10b981');
  });
}

function renderNetDrawer(titleEl, bodyEl) {
  titleEl.textContent = 'Network: Interface Telemetry (/proc/net/dev)';

  bodyEl.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">eth0: 2.5 GbE Primary LAN Interface</div>
      <div class="drawer-grid">
        <div class="drawer-row">
          <span class="drawer-row-lbl">Receive Throughput (RX Rate)</span>
          <span class="drawer-row-val text-net">${rxBuffer[rxBuffer.length - 1].toFixed(1)} MB/s</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">Transmit Throughput (TX Rate)</span>
          <span class="drawer-row-val text-disk">${txBuffer[txBuffer.length - 1].toFixed(1)} MB/s</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">Packet Drops</span>
          <span class="drawer-row-val text-ram">0 pkts/s</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">MTU Configuration</span>
          <span class="drawer-row-val">1500 Bytes</span>
        </div>
      </div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">RX Traffic History</div>
      <canvas class="sparkline-canvas" id="spark-rx"></canvas>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">TX Traffic History</div>
      <canvas class="sparkline-canvas" id="spark-tx"></canvas>
    </div>
  `;

  requestAnimationFrame(() => {
    drawSparkline(document.getElementById('spark-rx'), rxBuffer, '#38bdf8');
    drawSparkline(document.getElementById('spark-tx'), txBuffer, '#a78bfa');
  });
}

function renderDiskDrawer(titleEl, bodyEl) {
  titleEl.textContent = 'Storage: Local Pools and I/O (/proc/diskstats)';

  bodyEl.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Mounted Storage Pools</div>
      <div class="drawer-grid">
        <div class="drawer-row">
          <span class="drawer-row-lbl">/dev/nvme0n1p2 (/)</span>
          <span class="drawer-row-val">174 GB / 500 GB (34.8%)</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">/dev/sda1 (/mnt/data)</span>
          <span class="drawer-row-val">964 GB / 2 TB (48.2%)</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">/dev/sdb1 (/mnt/tank)</span>
          <span class="drawer-row-val">5.4 TB / 8 TB (68.5%)</span>
        </div>
      </div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">NVMe Storage I/O Rates</div>
      <div class="drawer-grid">
        <div class="drawer-row">
          <span class="drawer-row-lbl">Read Bandwidth</span>
          <span class="drawer-row-val text-net">2.4 MB/s</span>
        </div>
        <div class="drawer-row">
          <span class="drawer-row-lbl">Write Bandwidth</span>
          <span class="drawer-row-val text-disk">480 KB/s</span>
        </div>
      </div>
    </div>
  `;
}

function renderProcDrawer(titleEl, bodyEl, context) {
  const pid  = context?.pid  || '1042';
  const meta = HOMELAB_SERVICES[pid] || {
    name: context?.name || 'statix',
    cmdline: `/usr/local/bin/${context?.name || 'statix'}`,
    user: context?.user || 'statix',
    uid: '1001',
    gid: '1001',
    state: 'S (Interruptible Sleep)',
    cpu: context?.cpu || '0.1',
    rss: context?.rss || '18.2 MB',
    vmsize: '28.4 MB',
    threads: 6,
    fds: 18,
    statusText: `Name:\t${context?.name || 'statix'}
State:\tS (sleeping)
Pid:\t${pid}
VmSize:\t   28410 kB
VmRSS:\t   ${context?.rss || '18240 kB'}
Threads:\t6`
  };

  titleEl.textContent = `Process Inspector: ${meta.name} (PID: ${pid})`;

  bodyEl.innerHTML = `
    <div class="drawer-section">
      <div class="drawer-section-title">Process Identity and Resource Allocation</div>
      <div class="drawer-grid">
        <div class="drawer-row"><span class="drawer-row-lbl">PID</span><span class="drawer-row-val">${pid}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">Service</span><span class="drawer-row-val text-proc">${meta.name}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">User (UID:GID)</span><span class="drawer-row-val">${meta.user} (${meta.uid}:${meta.gid})</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">State</span><span class="drawer-row-val">${meta.state}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">CPU Utilization</span><span class="drawer-row-val text-cpu">${meta.cpu}%</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">Physical Memory (VmRSS)</span><span class="drawer-row-val text-ram">${meta.rss}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">Virtual Memory (VmSize)</span><span class="drawer-row-val">${meta.vmsize}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">Active Threads</span><span class="drawer-row-val">${meta.threads}</span></div>
        <div class="drawer-row"><span class="drawer-row-lbl">Open File Descriptors (FDs)</span><span class="drawer-row-val">${meta.fds}</span></div>
      </div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">Execution Command</div>
      <div class="raw-proc-box text-mono">${meta.cmdline}</div>
    </div>

    <div class="drawer-section">
      <div class="drawer-section-title">Kernel /proc/${pid}/status Header Extract</div>
      <div class="raw-proc-box">${meta.statusText}</div>
    </div>
  `;
}
