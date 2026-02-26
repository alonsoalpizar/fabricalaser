/* ================================================
   FABRICALASER ADMIN - Common JavaScript
   ================================================ */

const API_BASE = '/api/v1';

// ================================================
// STATE
// ================================================
const adminState = {
  user: null,
  token: localStorage.getItem('fl_token'),
  currentPage: 'dashboard'
};

// ================================================
// AUTH
// ================================================
async function checkAuth() {
  if (!adminState.token) {
    redirectToLogin();
    return false;
  }

  try {
    const res = await fetch(`${API_BASE}/auth/me`, {
      headers: { 'Authorization': `Bearer ${adminState.token}` }
    });

    if (!res.ok) {
      redirectToLogin();
      return false;
    }

    const data = await res.json();

    // API returns { data: { usuario: {...} } } format
    const user = data.data?.usuario || data.usuario || data.data || data;
    if (!user || !user.id) {
      console.error('Invalid user data:', data);
      redirectToLogin();
      return false;
    }

    adminState.user = user;

    // Check if user is admin
    if (adminState.user.role !== 'admin') {
      showToast('No tienes permisos de administrador', 'error');
      setTimeout(() => {
        window.location.href = '/mi-cuenta/';
      }, 1500);
      return false;
    }

    updateUserDisplay();
    return true;
  } catch (err) {
    console.error('Auth check failed:', err);
    redirectToLogin();
    return false;
  }
}

function redirectToLogin() {
  localStorage.removeItem('fl_token');
  window.location.href = '/landing/';
}

function logout() {
  localStorage.removeItem('fl_token');
  window.location.href = '/landing/';
}

function updateUserDisplay() {
  const user = adminState.user;
  if (!user) return;

  // Get initials
  const names = (user.nombre || '').split(' ');
  const initials = names.length >= 2
    ? (names[0][0] + names[names.length - 1][0]).toUpperCase()
    : (user.nombre || 'A').substring(0, 2).toUpperCase();

  // Update sidebar user
  const avatarEl = document.querySelector('.sidebar-avatar');
  const nameEl = document.querySelector('.sidebar-user-name');
  const roleEl = document.querySelector('.sidebar-user-role');

  if (avatarEl) avatarEl.textContent = initials;
  if (nameEl) nameEl.textContent = user.nombre || 'Admin';
  if (roleEl) roleEl.textContent = user.role === 'admin' ? 'Administrador' : 'Usuario';
}

// ================================================
// API HELPERS
// ================================================
async function apiGet(endpoint) {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    headers: {
      'Authorization': `Bearer ${adminState.token}`,
      'Content-Type': 'application/json'
    }
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error?.message || 'Error en la solicitud');
  }
  return data;
}

async function apiPost(endpoint, body) {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${adminState.token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error?.message || 'Error en la solicitud');
  }
  return data;
}

async function apiPut(endpoint, body) {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    method: 'PUT',
    headers: {
      'Authorization': `Bearer ${adminState.token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error?.message || 'Error en la solicitud');
  }
  return data;
}

async function apiDelete(endpoint) {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${adminState.token}`
    }
  });
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error?.message || 'Error en la solicitud');
  }
  return true;
}

// ================================================
// TOASTS
// ================================================
function showToast(message, type = 'info') {
  let container = document.querySelector('.toast-container');
  if (!container) {
    container = document.createElement('div');
    container.className = 'toast-container';
    document.body.appendChild(container);
  }

  const icons = {
    success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>',
    error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>',
    warning: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>',
    info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>'
  };

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.innerHTML = `
    <span class="toast-icon">${icons[type]}</span>
    <span class="toast-message">${message}</span>
    <button class="toast-close" onclick="this.parentElement.remove()">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <line x1="18" y1="6" x2="6" y2="18"></line>
        <line x1="6" y1="6" x2="18" y2="18"></line>
      </svg>
    </button>
  `;

  container.appendChild(toast);

  // Auto remove after 5 seconds
  setTimeout(() => {
    toast.style.animation = 'slideIn 0.3s ease reverse';
    setTimeout(() => toast.remove(), 300);
  }, 5000);
}

// ================================================
// MODALS
// ================================================
function openModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.add('active');
    document.body.style.overflow = 'hidden';
  }
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove('active');
    document.body.style.overflow = '';
  }
}

function closeAllModals() {
  document.querySelectorAll('.modal-overlay').forEach(m => {
    m.classList.remove('active');
  });
  document.body.style.overflow = '';
}

// Close modal on overlay click
document.addEventListener('click', (e) => {
  if (e.target.classList.contains('modal-overlay')) {
    closeAllModals();
  }
});

// Close modal on Escape key
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    closeAllModals();
  }
});

// ================================================
// SIDEBAR NAVIGATION
// ================================================
function setActiveNav(page) {
  document.querySelectorAll('.nav-item').forEach(item => {
    item.classList.remove('active');
    if (item.dataset.page === page) {
      item.classList.add('active');
    }
  });
}

function toggleSubmenu(sectionId) {
  const submenu = document.getElementById(sectionId);
  if (submenu) {
    submenu.classList.toggle('hidden');
  }
}

// ================================================
// PAGINATION HELPERS
// ================================================
function renderPagination(current, total, onPageChange) {
  const pages = [];
  const maxVisible = 5;

  let start = Math.max(1, current - Math.floor(maxVisible / 2));
  let end = Math.min(total, start + maxVisible - 1);

  if (end - start + 1 < maxVisible) {
    start = Math.max(1, end - maxVisible + 1);
  }

  // Previous button
  pages.push(`<button class="pagination-btn" ${current === 1 ? 'disabled' : ''} onclick="${onPageChange}(${current - 1})">
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <polyline points="15 18 9 12 15 6"></polyline>
    </svg>
  </button>`);

  // Page numbers
  for (let i = start; i <= end; i++) {
    pages.push(`<button class="pagination-btn ${i === current ? 'active' : ''}" onclick="${onPageChange}(${i})">${i}</button>`);
  }

  // Next button
  pages.push(`<button class="pagination-btn" ${current === total ? 'disabled' : ''} onclick="${onPageChange}(${current + 1})">
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <polyline points="9 18 15 12 9 6"></polyline>
    </svg>
  </button>`);

  return `
    <div class="pagination">
      ${pages.join('')}
      <span class="pagination-info">Pagina ${current} de ${total}</span>
    </div>
  `;
}

// ================================================
// FORMATTING HELPERS
// ================================================
function formatCurrency(amount) {
  return new Intl.NumberFormat('es-CR', {
    style: 'currency',
    currency: 'CRC',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
}

function formatDate(dateStr) {
  if (!dateStr) return '-';
  const date = new Date(dateStr);
  return new Intl.DateTimeFormat('es-CR', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date);
}

function formatDateShort(dateStr) {
  if (!dateStr) return '-';
  const date = new Date(dateStr);
  return new Intl.DateTimeFormat('es-CR', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }).format(date);
}

function truncate(str, len = 30) {
  if (!str) return '';
  return str.length > len ? str.substring(0, len) + '...' : str;
}

// ================================================
// CONFIRM DIALOG
// ================================================
function confirmAction(message) {
  return new Promise((resolve) => {
    // For now use native confirm, can be replaced with custom modal
    resolve(confirm(message));
  });
}

// ================================================
// DEBOUNCE
// ================================================
function debounce(func, wait) {
  let timeout;
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
}

// ================================================
// LOADING STATES
// ================================================
function showLoading(containerId) {
  const container = document.getElementById(containerId);
  if (container) {
    container.innerHTML = `
      <div class="loading">
        <div class="spinner"></div>
      </div>
    `;
  }
}

function showEmpty(containerId, message = 'No hay datos disponibles') {
  const container = document.getElementById(containerId);
  if (container) {
    container.innerHTML = `
      <div class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
          <polyline points="17 8 12 3 7 8"></polyline>
          <line x1="12" y1="3" x2="12" y2="15"></line>
        </svg>
        <p>${message}</p>
      </div>
    `;
  }
}

// ================================================
// SIDEBAR ICONS (SVG)
// ================================================
const icons = {
  dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"></rect><rect x="14" y="3" width="7" height="7"></rect><rect x="14" y="14" width="7" height="7"></rect><rect x="3" y="14" width="7" height="7"></rect></svg>',
  users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>',
  quotes: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>',
  settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>',
  tech: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L2 7l10 5 10-5-10-5z"></path><path d="M2 17l10 5 10-5"></path><path d="M2 12l10 5 10-5"></path></svg>',
  materials: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>',
  engrave: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19l7-7 3 3-7 7-3-3z"></path><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"></path><path d="M2 2l7.586 7.586"></path><circle cx="11" cy="11" r="2"></circle></svg>',
  rates: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"></line><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>',
  discounts: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="9" cy="9" r="2"></circle><circle cx="15" cy="15" r="2"></circle><line x1="5" y1="19" x2="19" y2="5"></line></svg>',
  logout: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline points="16 17 21 12 16 7"></polyline><line x1="21" y1="12" x2="9" y2="12"></line></svg>',
  chevronDown: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg>',
  edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>',
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>',
  eye: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>',
  check: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"></polyline></svg>',
  x: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>'
};

// ================================================
// RENDER SIDEBAR
// ================================================
function renderSidebar(activePage = 'dashboard') {
  const sidebarEl = document.getElementById('sidebar');
  if (!sidebarEl) return;

  const configPages = ['technologies', 'materials', 'material-costs', 'engrave-types', 'rates', 'discounts', 'general', 'speeds'];
  const isConfigActive = configPages.includes(activePage);

  sidebarEl.innerHTML = `
    <div class="sidebar-header">
      <a href="/admin/" class="sidebar-logo">
        FABRICA<span>LASER</span>
        <span class="sidebar-badge">Admin</span>
      </a>
    </div>

    <nav class="sidebar-nav">
      <div class="nav-section">
        <a href="/admin/" class="nav-item ${activePage === 'dashboard' ? 'active' : ''}" data-page="dashboard">
          ${icons.dashboard}
          <span>Dashboard</span>
        </a>
        <a href="/admin/users.html" class="nav-item ${activePage === 'users' ? 'active' : ''}" data-page="users">
          ${icons.users}
          <span>Usuarios</span>
        </a>
        <a href="/admin/quotes.html" class="nav-item ${activePage === 'quotes' ? 'active' : ''}" data-page="quotes">
          ${icons.quotes}
          <span>Cotizaciones</span>
          <span class="badge" id="pendingBadge" style="display:none">0</span>
        </a>
      </div>

      <div class="nav-section">
        <div class="nav-section-title">Configuracion</div>
        <a href="/admin/config/general.html" class="nav-item ${activePage === 'general' ? 'active' : ''}" data-page="general">
          ${icons.settings}
          <span>General</span>
        </a>
        <a href="/admin/config/technologies.html" class="nav-item ${activePage === 'technologies' ? 'active' : ''}" data-page="technologies">
          ${icons.tech}
          <span>Tecnologias</span>
        </a>
        <a href="/admin/config/materials.html" class="nav-item ${activePage === 'materials' ? 'active' : ''}" data-page="materials">
          ${icons.materials}
          <span>Materiales</span>
        </a>
        <a href="/admin/config/material-costs.html" class="nav-item ${activePage === 'material-costs' ? 'active' : ''}" data-page="material-costs">
          ${icons.rates}
          <span>Costos Material</span>
        </a>
        <a href="/admin/config/engrave-types.html" class="nav-item ${activePage === 'engrave-types' ? 'active' : ''}" data-page="engrave-types">
          ${icons.engrave}
          <span>Tipos de Grabado</span>
        </a>
        <a href="/admin/config/rates.html" class="nav-item ${activePage === 'rates' ? 'active' : ''}" data-page="rates">
          ${icons.rates}
          <span>Tarifas</span>
        </a>
        <a href="/admin/config/discounts.html" class="nav-item ${activePage === 'discounts' ? 'active' : ''}" data-page="discounts">
          ${icons.discounts}
          <span>Descuentos</span>
        </a>
        <a href="/admin/config/speeds.html" class="nav-item ${activePage === 'speeds' ? 'active' : ''}" data-page="speeds">
          ${icons.tech}
          <span>Velocidades</span>
        </a>
      </div>
    </nav>

    <div class="sidebar-footer">
      <div class="sidebar-user">
        <div class="sidebar-avatar">AD</div>
        <div class="sidebar-user-info">
          <div class="sidebar-user-name">Admin</div>
          <div class="sidebar-user-role">Administrador</div>
        </div>
        <button class="sidebar-logout" onclick="logout()" title="Cerrar sesion">
          ${icons.logout}
        </button>
      </div>
    </div>
  `;
}

// ================================================
// INIT
// ================================================
async function initAdmin(page = 'dashboard') {
  adminState.currentPage = page;

  // Check auth first
  const isAuthed = await checkAuth();
  if (!isAuthed) return false;

  // Render sidebar
  renderSidebar(page);

  // Load pending quotes count for badge
  try {
    const quotesData = await apiGet('/admin/quotes?status=pending&limit=1');
    if (quotesData.data && quotesData.data.total > 0) {
      const badge = document.getElementById('pendingBadge');
      if (badge) {
        badge.textContent = quotesData.data.total;
        badge.style.display = 'block';
      }
    }
  } catch (err) {
    console.error('Failed to load pending count:', err);
  }

  return true;
}
