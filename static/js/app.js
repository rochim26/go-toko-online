// HTMX is loaded separately. This file glues client-side tracking.
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

  // Cart count update via HTMX events
  document.body.addEventListener('cart:updated', (e) => {
    const cnt = (e.detail||{}).count;
    const el = document.getElementById('cart-count');
    if (el && typeof cnt === 'number') {
      el.textContent = cnt;
      el.style.display = cnt > 0 ? 'inline-flex' : 'none';
    }
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
