(() => {
  const ic = (d, sw = 1.6) => `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${sw}" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${d}</svg>`;
  const I = {
    spark: ic('<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5L12 3Z"/>'),
    close: ic('<path d="M6 6l12 12M18 6 6 18"/>'),
    plus: ic('<path d="M12 5v14M5 12h14"/>'),
    arrow: ic('<path d="M5 12h14m-6-6 6 6-6 6"/>'),
    up: ic('<path d="M12 19V5m-6 6 6-6 6 6"/>', 2),
    book: ic('<path d="M12 5v16m0-16C8 2 4 3 2 4v15c4-2 7-1 10 2 3-3 6-4 10-2V4c-3-1-6-2-10 1Z"/>'),
  };
  const LOGO = document.currentScript?.dataset.logo || 'uploads/logo.png';
  const esc = s => s.replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

  const KB = [
    { k: ['remote', 'connect', 'session', 'help a merchant'], text: 'Remote Connect lets you start a live support session on a merchant’s terminal from Terminal Management.', steps: ['Open Terminal Management › Find, add, and manage devices.', 'Find the device by serial number or terminal ID.', 'Select Remote Connect, then start the session.', 'Wait for the merchant to accept on the device.'], q: 'remote connect' },
    { k: ['transaction', 'failed', 'payment', 'card', 'rrn'], text: 'Find any card transaction from Transactions using its RRN, STAN, or masked PAN, then open it to see the response code and route.', steps: ['Go to Transactions › Find and inspect card transactions.', 'Set the date range, then search by RRN or STAN.', 'Open the row to see the response code and processor route.'], q: 'card transactions' },
    { k: ['invite', 'user', 'staff', 'role', 'access'], text: 'Staff users are invited from User Management. They get an email to set their password, and their role controls what they can see.', steps: ['Open User Management › Invite a staff user.', 'Enter their work email and choose a role.', 'Send the invite.'], q: 'invite user roles' },
    { k: ['report', 'financial', 'export'], text: 'Financial reports are created from Reports and appear in Recent reports when they’re ready to download.', steps: ['Go to Reports › Create a report.', 'Pick Financial Report, an institution, and a date range.', 'Generate it, then download from Recent reports.'], q: 'create report financial' },
    { k: ['terminal', 'pos', 'device', 'troubleshoot', 'bind'], text: 'Most POS issues can be diagnosed from the device’s information and history before taking a support action.', steps: ['Open Terminal Management › Troubleshoot a POS device or terminal.', 'Check the device’s last activity and binding.', 'Use a device support action if it needs a reset.'], q: 'troubleshoot device' },
  ];
  const SUGGEST = ['How can I help a merchant remotely?', 'Why did a card transaction fail?', 'How do I invite a staff user?', 'How do I create a financial report?'];

  const panel = document.createElement('aside');
  panel.className = 'ai-panel';
  panel.setAttribute('aria-label', 'Ask AI');
  panel.innerHTML = `
    <div class="ai-head"><span class="ai-head-glyph">${I.spark}</span><div class="ai-head-text"><h2>Ask AI</h2></div>
      <div class="ai-head-actions">
        <button class="ai-icon-btn" data-ai-new title="New chat" aria-label="New chat" hidden>${I.plus}</button>
        <button class="ai-icon-btn" data-ai-close title="Close (Esc)" aria-label="Close Ask AI">${I.close}</button>
      </div></div>
    <div class="ai-body">
      <div class="ai-empty">
        <div><span class="eyebrow">ASK THE DOCUMENTATION</span><h3>How can I help?</h3><p>Ask in your own words. Answers come from these guides, with a link to each source.</p></div>
        <div class="ai-suggest">${SUGGEST.map(q => `<button data-ai-q="${esc(q)}"><span>${esc(q)}</span>${I.arrow}</button>`).join('')}</div>
      </div>
      <div class="ai-thread" hidden></div>
    </div>
    <div class="ai-foot"><div class="ai-input">
      <textarea rows="2" placeholder="Ask anything…" aria-label="Ask a question"></textarea>
      <div class="ai-input-row"><span class="ai-note" tabindex="0">AI can make mistakes ${ic('<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>')}<span class="ai-tip" role="tooltip">Answers are AI-generated from these guides and may be incomplete or wrong. Check the linked sources before acting.</span></span><button class="ai-send" aria-label="Send" disabled>${I.up}</button></div>
    </div></div>`;
  document.body.appendChild(panel);

  const $ = s => panel.querySelector(s);
  const body = $('.ai-body'), empty = $('.ai-empty'), thread = $('.ai-thread'), ta = $('textarea'), send = $('.ai-send'), newBtn = $('[data-ai-new]');
  let busy = false, timer;

  const isOpen = () => document.body.classList.contains('ask-open');
  const sync = () => document.querySelectorAll('.header-ask').forEach(b => { b.setAttribute('aria-expanded', String(isOpen())); b.setAttribute('aria-label', 'Ask AI'); });
  const setOpen = (o) => { document.body.classList.toggle('ask-open', o); sync(); if (o) setTimeout(() => ta.focus(), 280); };

  // Take over every [data-ask] trigger before the app's "coming soon" dialog handler runs.
  document.addEventListener('click', e => {
    const t = e.target.closest('[data-ask]');
    if (!t) return;
    e.preventDefault(); e.stopPropagation();
    setOpen(t.classList.contains('header-ask') ? !isOpen() : true);
  }, true);
  document.addEventListener('keydown', e => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'i') { e.preventDefault(); setOpen(!isOpen()); }
    else if (e.key === 'Escape' && isOpen() && !document.querySelector('dialog[open]')) setOpen(false);
  });
  new MutationObserver(sync).observe(document.getElementById('app'), { childList: true });

  const findSources = (q) => {
    const words = q.toLowerCase().split(/\W+/).filter(w => w.length > 2);
    return [...document.querySelectorAll('.sidebar .nav-link')]
      .map(a => { const t = a.textContent.toLowerCase(); return { a, s: words.reduce((n, w) => n + (t.includes(w) ? 1 : 0), 0) }; })
      .filter(x => x.s > 0).sort((x, y) => y.s - x.s).slice(0, 3)
      .map(x => ({ href: x.a.getAttribute('href'), title: x.a.textContent.trim() }));
  };
  const scroll = () => { body.scrollTop = body.scrollHeight; };

  const ask = (q) => {
    q = q.trim(); if (!q || busy) return;
    busy = true; empty.hidden = true; thread.hidden = false; newBtn.hidden = false;
    ta.value = ''; send.disabled = true;
    thread.insertAdjacentHTML('beforeend', `<div class="ai-q">${esc(q)}</div><div class="ai-typing"><img class="ai-gear" src="${LOGO}" alt="" width="18" height="18">Searching the guides…</div>`);
    scroll();
    const lq = q.toLowerCase();
    const hit = KB.find(e => e.k.some(w => lq.includes(w)));
    timer = setTimeout(() => {
      thread.querySelector('.ai-typing')?.remove();
      const sources = findSources(hit ? hit.q : q);
      const text = hit ? hit.text : (sources.length ? 'I couldn’t find a direct answer, but these guides look closest to your question.' : 'I couldn’t find a guide for that. Try rephrasing, or use Search documentation.');
      const el = document.createElement('div');
      el.className = 'ai-a';
      el.innerHTML = `<p>${esc(text)}</p>${hit ? `<ol>${hit.steps.map(s => `<li>${esc(s)}</li>`).join('')}</ol>` : ''}
        ${sources.length ? `<div class="ai-sources"><span class="eyebrow">SOURCES</span>${sources.map(s => `<a href="${esc(s.href)}">${I.book}<span>${esc(s.title)}</span></a>`).join('')}</div>` : ''}
        <div class="ai-meta"><button data-copy>Copy</button><button data-vote="up" aria-pressed="false">Helpful</button><button data-vote="down" aria-pressed="false">Not helpful</button></div>`;
      el.querySelector('[data-copy]').onclick = (ev) => { navigator.clipboard?.writeText(el.querySelector('p').textContent + (hit ? '\n' + hit.steps.map((s, i) => `${i + 1}. ${s}`).join('\n') : '')); ev.currentTarget.textContent = 'Copied'; setTimeout(() => ev.currentTarget && (ev.currentTarget.textContent = 'Copy'), 1400); };
      el.querySelectorAll('[data-vote]').forEach(b => b.onclick = () => { const on = b.getAttribute('aria-pressed') !== 'true'; el.querySelectorAll('[data-vote]').forEach(x => x.setAttribute('aria-pressed', 'false')); b.setAttribute('aria-pressed', String(on)); });
      thread.appendChild(el);
      busy = false; send.disabled = !ta.value.trim(); scroll();
    }, 1100);
  };

  panel.querySelectorAll('[data-ai-q]').forEach(b => b.onclick = () => ask(b.dataset.aiQ));
  $('[data-ai-close]').onclick = () => setOpen(false);
  newBtn.onclick = () => { clearTimeout(timer); busy = false; thread.innerHTML = ''; thread.hidden = true; empty.hidden = false; newBtn.hidden = true; ta.focus(); };
  ta.oninput = () => { send.disabled = busy || !ta.value.trim(); };
  ta.onkeydown = e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); ask(ta.value); } };
  send.onclick = () => ask(ta.value);
})();
