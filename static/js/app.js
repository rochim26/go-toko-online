// HTMX is loaded separately. This file glues client-side tracking + UI.

// ── Toast / snackbar ─────────────────────────────────────
window.toast = function (msg, kind = '', ms = 2200) {
  let wrap = document.querySelector('.toast-wrap');
  if (!wrap) {
    wrap = document.createElement('div');
    wrap.className = 'toast-wrap';
    document.body.appendChild(wrap);
  }
  const t = document.createElement('div');
  t.className = 'toast ' + kind;
  t.textContent = msg;
  wrap.appendChild(t);
  setTimeout(() => t.classList.add('fading'), ms - 180);
  setTimeout(() => t.remove(), ms);
};

(function(){
  // Persist UTM on first visit
  try {
    const params = new URLSearchParams(window.location.search);
    const utmKeys = ['utm_source','utm_medium','utm_campaign','utm_term','utm_content','gclid','fbclid'];
    const captured = {};
    let any = false;
    utmKeys.forEach(k => {
      const v = params.get(k);
      if (v) { captured[k] = v; any = true; }
    });
    if (any) {
      const stored = JSON.parse(localStorage.getItem('mdt_attr') || '{}');
      Object.assign(stored, captured, { last_seen: Date.now() });
      if (!stored.first_touch) stored.first_touch = { ...captured, ts: Date.now(), referer: document.referrer };
      localStorage.setItem('mdt_attr', JSON.stringify(stored));
      // Send to server attribution endpoint
      fetch('/api/attribution', {
        method: 'POST',
        headers: {'Content-Type':'application/json'},
        body: JSON.stringify({ ...captured, landing: location.href, referer: document.referrer })
      }).catch(()=>{});
    }
  } catch(e){}

  // Service worker (PWA)
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/sw.js').catch(()=>{});
    });
  }

  // Cart count update via HTMX events + bottom-nav badge sync
  document.body.addEventListener('cart:updated', (e) => {
    const cnt = (e.detail||{}).count;
    const el = document.getElementById('cart-count');
    if (el && typeof cnt === 'number') {
      el.textContent = cnt > 99 ? '99+' : cnt;
      el.style.display = cnt > 0 ? 'inline-flex' : 'none';
    }
    // Bottom nav cart pill
    const navWrap = document.querySelector('.bottomnav a[href="/cart"] .icon-wrap');
    if (navWrap) {
      let pill = navWrap.querySelector('.pill');
      if (cnt > 0) {
        if (!pill) {
          pill = document.createElement('span');
          pill.className = 'pill';
          navWrap.appendChild(pill);
        }
        pill.textContent = cnt > 99 ? '99+' : cnt;
      } else if (pill) {
        pill.remove();
      }
    }
    if (typeof cnt === 'number' && cnt > 0) {
      window.toast?.('Ditambahkan ke keranjang ✓', 'success', 1800);
    }
  });

  // Auto-add toast for any HTMX 4xx/5xx
  document.body.addEventListener('htmx:responseError', (e) => {
    const status = e.detail?.xhr?.status;
    if (status >= 400) {
      window.toast?.('Terjadi kesalahan ('+status+'). Coba lagi.', 'error', 2400);
    }
  });

  // Auto-close drawer on navigation
  document.body.addEventListener('click', (e) => {
    if (e.target.closest('.sidebar a:not(.drawer-toggle)')) {
      document.body.classList.remove('drawer-open');
    }
  });

  // Wishlist heart toggle (visual only, persisted in localStorage)
  function loadWishlist(){
    try { return JSON.parse(localStorage.getItem('mdt_wishlist')||'[]'); } catch(e){ return []; }
  }
  function saveWishlist(list){
    try { localStorage.setItem('mdt_wishlist', JSON.stringify(list.slice(0,200))); } catch(e){}
  }
  function paintWishlist(){
    const list = loadWishlist();
    document.querySelectorAll('[data-wishlist]').forEach(el => {
      el.classList.toggle('active', list.indexOf(el.getAttribute('data-wishlist')) >= 0);
    });
  }
  paintWishlist();
  document.body.addEventListener('click', (e) => {
    const el = e.target.closest('[data-wishlist]');
    if (!el) return;
    e.preventDefault();
    e.stopPropagation();
    const slug = el.getAttribute('data-wishlist');
    const list = loadWishlist();
    const i = list.indexOf(slug);
    if (i >= 0) {
      list.splice(i, 1);
      el.classList.remove('active');
      window.toast?.('Dihapus dari wishlist', '', 1400);
    } else {
      list.push(slug);
      el.classList.add('active');
      window.toast?.('Disimpan ke wishlist ❤', 'success', 1400);
    }
    saveWishlist(list);
  });

  // Mobile nav toggle
  const navBtn = document.querySelector('[data-nav-toggle]');
  if (navBtn) {
    navBtn.addEventListener('click', () => document.body.classList.toggle('nav-open'));
  }

  // Track ViewContent on product page (event_id from server, dedup with CAPI)
  if (window.__mdt_event) {
    window.__mdt_event.forEach(ev => {
      try { sendClientEvent(ev); } catch(e){}
    });
  }

  function sendClientEvent(ev) {
    if (window.fbq && ev.fb_name) {
      window.fbq('track', ev.fb_name, ev.params || {}, { eventID: ev.event_id });
    }
    if (window.gtag && ev.ga_name) {
      window.gtag('event', ev.ga_name, Object.assign({event_id: ev.event_id}, ev.params || {}));
    }
  }
})();
