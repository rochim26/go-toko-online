// Lightweight in-app tour. Activated when window.__mdt_tour is present.
//
// Each step targets a CSS selector or virtual position. Steps are defined in
// HTML data attributes (data-tour="key" data-tour-title="…" data-tour-text="…"
// data-tour-order="N") OR injected via window.__mdt_tour_steps.
(function () {
  if (!window.__mdt_tour && !window.__mdt_tour_force) return;

  const stepsFromDOM = () => {
    const els = document.querySelectorAll('[data-tour]');
    const arr = [];
    els.forEach(e => {
      arr.push({
        el: e,
        title: e.dataset.tourTitle || '',
        text: e.dataset.tourText || '',
        order: parseInt(e.dataset.tourOrder || '0', 10),
      });
    });
    arr.sort((a, b) => a.order - b.order);
    return arr;
  };

  const stepsExtra = window.__mdt_tour_steps || [];
  const steps = [...stepsFromDOM(), ...stepsExtra];

  if (!steps.length) return;

  // Build overlay
  const overlay = document.createElement('div');
  overlay.className = 'tour-overlay';
  overlay.innerHTML = `
    <div class="tour-spotlight"></div>
    <div class="tour-pop">
      <div class="tour-step-num"></div>
      <h3 class="tour-title"></h3>
      <p class="tour-text"></p>
      <div class="tour-actions">
        <button type="button" class="tour-skip">Lewati Tour</button>
        <div>
          <button type="button" class="tour-prev">Sebelumnya</button>
          <button type="button" class="tour-next">Selanjutnya</button>
        </div>
      </div>
    </div>
  `;
  document.body.appendChild(overlay);

  let idx = 0;
  const spot = overlay.querySelector('.tour-spotlight');
  const pop = overlay.querySelector('.tour-pop');
  const tStep = overlay.querySelector('.tour-step-num');
  const tTitle = overlay.querySelector('.tour-title');
  const tText = overlay.querySelector('.tour-text');
  const bPrev = overlay.querySelector('.tour-prev');
  const bNext = overlay.querySelector('.tour-next');
  const bSkip = overlay.querySelector('.tour-skip');

  function show(i) {
    if (i < 0 || i >= steps.length) {
      finish();
      return;
    }
    const s = steps[i];
    const el = s.el || (s.selector ? document.querySelector(s.selector) : null);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' });
      requestAnimationFrame(() => {
        const r = el.getBoundingClientRect();
        const pad = 8;
        spot.style.top = (r.top - pad) + 'px';
        spot.style.left = (r.left - pad) + 'px';
        spot.style.width = (r.width + pad * 2) + 'px';
        spot.style.height = (r.height + pad * 2) + 'px';
        // Place popover below if room, otherwise above
        const popH = 220, popW = Math.min(380, window.innerWidth - 32);
        let top = r.bottom + 16;
        if (top + popH > window.innerHeight) top = Math.max(16, r.top - popH - 16);
        let left = Math.max(16, Math.min(window.innerWidth - popW - 16, r.left));
        pop.style.top = top + 'px';
        pop.style.left = left + 'px';
        pop.style.width = popW + 'px';
      });
    } else {
      // Centered popover with no spotlight
      spot.style.width = '0px';
      spot.style.height = '0px';
      pop.style.top = '50%';
      pop.style.left = '50%';
      pop.style.transform = 'translate(-50%, -50%)';
    }
    tStep.textContent = `Langkah ${i + 1} / ${steps.length}`;
    tTitle.textContent = s.title;
    tText.innerHTML = s.text;
    bPrev.style.visibility = i === 0 ? 'hidden' : 'visible';
    bNext.textContent = i === steps.length - 1 ? 'Selesai' : 'Selanjutnya →';
  }

  function finish() {
    overlay.remove();
    // mark completed on server (best-effort)
    if (window.__mdt_tour_csrf) {
      fetch('/admin/onboarding/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'csrf_token=' + encodeURIComponent(window.__mdt_tour_csrf),
        credentials: 'include',
      }).catch(() => {});
    }
  }

  bPrev.addEventListener('click', () => show(--idx));
  bNext.addEventListener('click', () => show(++idx));
  bSkip.addEventListener('click', finish);
  window.addEventListener('resize', () => show(idx));
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') finish();
    if (e.key === 'ArrowRight') show(++idx);
    if (e.key === 'ArrowLeft') show(--idx);
  });

  // small delay for layout
  setTimeout(() => show(0), 250);
})();
