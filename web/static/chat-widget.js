(function() {
  'use strict';

  let isOpen = false;
  let isLoading = false;
  let chatHistory = [];
  let cachedWaSummary = null;
  let waLinkShown = false;

  // ===== CREATE DOM ELEMENTS =====
  const style = document.createElement('style');
  style.textContent = `
    #fl-chat-bubble {
      position: fixed;
      bottom: 24px;
      right: 24px;
      width: 56px;
      height: 56px;
      border-radius: 50%;
      background: linear-gradient(135deg, #9B2020, #C0392B);
      border: none;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      box-shadow: 0 4px 20px rgba(155, 32, 32, 0.4);
      z-index: 10000;
      transition: transform 0.2s, box-shadow 0.2s;
    }
    #fl-chat-bubble:hover {
      transform: scale(1.08);
      box-shadow: 0 6px 28px rgba(155, 32, 32, 0.5);
    }
    #fl-chat-bubble svg { width: 26px; height: 26px; fill: #fff; }

    #fl-chat-container {
      position: fixed;
      bottom: 92px;
      right: 24px;
      width: 380px;
      height: 520px;
      background: #1A1A1A;
      border: 1px solid #2A2A2A;
      border-radius: 16px;
      display: none;
      flex-direction: column;
      overflow: hidden;
      z-index: 10000;
      box-shadow: 0 12px 40px rgba(0,0,0,0.5);
      animation: fl-chat-slide-up 0.3s ease;
    }
    @keyframes fl-chat-slide-up {
      from { opacity: 0; transform: translateY(20px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    #fl-chat-header {
      background: linear-gradient(135deg, #9B2020, #7A1818);
      padding: 16px;
      display: flex;
      align-items: center;
      gap: 10px;
    }
    #fl-chat-header-icon {
      width: 36px; height: 36px;
      background: rgba(255,255,255,0.15);
      border-radius: 50%;
      display: flex; align-items: center; justify-content: center;
    }
    #fl-chat-header-icon svg { width: 20px; height: 20px; fill: #fff; }
    #fl-chat-header-info h4 {
      color: #fff; font-size: 14px; font-weight: 600; margin: 0;
      font-family: 'Inter', sans-serif;
    }
    #fl-chat-header-info span {
      color: rgba(255,255,255,0.7); font-size: 12px;
      font-family: 'Inter', sans-serif;
    }

    #fl-chat-messages {
      flex: 1;
      overflow-y: auto;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    #fl-chat-messages::-webkit-scrollbar { width: 4px; }
    #fl-chat-messages::-webkit-scrollbar-track { background: transparent; }
    #fl-chat-messages::-webkit-scrollbar-thumb { background: #333; border-radius: 2px; }

    .fl-msg {
      max-width: 85%;
      padding: 10px 14px;
      border-radius: 12px;
      font-size: 13px;
      line-height: 1.5;
      font-family: 'Inter', sans-serif;
      word-wrap: break-word;
    }
    .fl-msg a { color: #E8A0A0; text-decoration: underline; }
    .fl-msg-user {
      align-self: flex-end;
      background: #9B2020;
      color: #fff;
      border-bottom-right-radius: 4px;
    }
    .fl-msg-assistant {
      align-self: flex-start;
      background: #2A2A2A;
      color: #E0E0E0;
      border-bottom-left-radius: 4px;
    }

    .fl-typing {
      align-self: flex-start;
      display: flex;
      gap: 4px;
      padding: 12px 16px;
      background: #2A2A2A;
      border-radius: 12px;
      border-bottom-left-radius: 4px;
    }
    .fl-typing span {
      width: 6px; height: 6px;
      background: #666;
      border-radius: 50%;
      animation: fl-bounce 1.2s infinite;
    }
    .fl-typing span:nth-child(2) { animation-delay: 0.15s; }
    .fl-typing span:nth-child(3) { animation-delay: 0.3s; }
    @keyframes fl-bounce {
      0%, 60%, 100% { transform: translateY(0); }
      30% { transform: translateY(-6px); }
    }

    #fl-chat-input-area {
      padding: 12px;
      border-top: 1px solid #2A2A2A;
      display: flex;
      gap: 8px;
      background: #111;
    }
    #fl-chat-input {
      flex: 1;
      background: #1A1A1A;
      border: 1px solid #333;
      border-radius: 8px;
      padding: 10px 12px;
      color: #fff;
      font-size: 13px;
      font-family: 'Inter', sans-serif;
      outline: none;
      resize: none;
    }
    #fl-chat-input::placeholder { color: #666; }
    #fl-chat-input:focus { border-color: #9B2020; }
    #fl-chat-send {
      background: #9B2020;
      border: none;
      border-radius: 8px;
      width: 40px;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: background 0.2s;
    }
    #fl-chat-send:hover { background: #B52525; }
    #fl-chat-send:disabled { background: #333; cursor: not-allowed; }
    #fl-chat-send svg { width: 18px; height: 18px; fill: #fff; }

    @media (max-width: 480px) {
      #fl-chat-container {
        width: calc(100vw - 16px);
        height: calc(100vh - 100px);
        right: 8px;
        bottom: 80px;
        border-radius: 12px;
      }
    }
  `;
  document.head.appendChild(style);

  // Chat bubble
  const bubble = document.createElement('button');
  bubble.id = 'fl-chat-bubble';
  bubble.title = 'Chateá con nosotros';
  bubble.innerHTML = `<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>`;
  document.body.appendChild(bubble);

  // Chat container
  const container = document.createElement('div');
  container.id = 'fl-chat-container';
  container.innerHTML = `
    <div id="fl-chat-header">
      <div id="fl-chat-header-icon">
        <svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>
      </div>
      <div id="fl-chat-header-info">
        <h4>Asistente FabricaLaser</h4>
        <span>Corte y grabado l&aacute;ser &bull; Costa Rica</span>
      </div>
    </div>
    <div id="fl-chat-messages"></div>
    <div id="fl-chat-input-area">
      <input type="text" id="fl-chat-input" placeholder="Escrib&iacute; tu consulta..." maxlength="500" autocomplete="off">
      <button id="fl-chat-send" title="Enviar">
        <svg viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
      </button>
    </div>
  `;
  document.body.appendChild(container);

  const messagesDiv = document.getElementById('fl-chat-messages');
  const input = document.getElementById('fl-chat-input');
  const sendBtn = document.getElementById('fl-chat-send');

  // ===== OPEN CHAT (public API) =====
  function openChat(autoMessage) {
    if (!isOpen) {
      isOpen = true;
      container.style.display = 'flex';
      bubble.innerHTML = `<svg viewBox="0 0 24 24"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>`;
      if (chatHistory.length === 0) showWelcome();
    }
    if (autoMessage && chatHistory.length <= 1) {
      input.value = autoMessage;
      sendMessage();
    } else {
      input.focus();
    }
  }

  // Expose globally so buttons can call it
  window.flOpenChat = openChat;

  // ===== TOGGLE CHAT =====
  bubble.addEventListener('click', function() {
    if (isOpen) {
      isOpen = false;
      container.style.display = 'none';
      bubble.innerHTML = `<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>`;
    } else {
      openChat();
    }
  });

  // ===== WELCOME MESSAGE =====
  function showWelcome() {
    appendMessage('assistant', '¡Pura vida! ¿En qué te puedo ayudar?\n\nPuedo contarte sobre:\n- **Llaveros y medallas** de acrílico para personalizar\n- **Precios y cantidades** disponibles\n- **Cotizaciones** de corte y grabado láser con tu propio diseño\n- **Cómo hacer tu pedido**');
  }

  // ===== SEND MESSAGE =====
  function sendMessage() {
    const text = input.value.trim();
    if (!text || isLoading) return;

    appendMessage('user', text);
    input.value = '';
    isLoading = true;
    sendBtn.disabled = true;

    showTyping();

    var token = localStorage.getItem('fl_token');
    var headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = 'Bearer ' + token;

    fetch('/api/v1/chat/', {
      method: 'POST',
      headers: headers,
      body: JSON.stringify({
        message: text,
        history: chatHistory.slice(-10)
      })
    })
    .then(function(res) { return res.json(); })
    .then(function(data) {
      hideTyping();
      if (data.error) {
        appendMessage('assistant', 'Tuve un problema, intentá de nuevo.');
      } else {
        appendMessage('assistant', data.response);
        // Agregar ambos turnos al historial una vez que el intercambio está completo
        chatHistory.push({ role: 'user', content: text });
        chatHistory.push({ role: 'assistant', content: data.response });
        // Detectar primera aparición del link de WhatsApp
        if (!waLinkShown && data.response.indexOf('wa.me') !== -1) {
          waLinkShown = true;
        }
        // Refrescar resumen después de cada turno si el link ya fue mostrado
        if (waLinkShown) {
          prefetchWaSummary();
        }
      }
    })
    .catch(function() {
      hideTyping();
      appendMessage('assistant', 'No se pudo conectar. Intentá de nuevo en un momento.');
    })
    .finally(function() {
      isLoading = false;
      sendBtn.disabled = false;
      input.focus();
    });
  }

  sendBtn.addEventListener('click', sendMessage);
  input.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  // ===== HELPERS =====
  function appendMessage(role, content) {
    var div = document.createElement('div');
    div.className = 'fl-msg fl-msg-' + role;
    div.innerHTML = formatMarkdown(content);
    messagesDiv.appendChild(div);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
  }

  function formatMarkdown(text) {
    return text
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.*?)\*/g, '<em>$1</em>')
      .replace(/\[([^\]]+)\]\((https?:\/\/[^\)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
      .replace(/(^|[\s>])(https?:\/\/[^\s<)\].,'"]+)/g, '$1<a href="$2" target="_blank" rel="noopener">$2</a>')
      .replace(/\n/g, '<br>');
  }

  function showTyping() {
    var div = document.createElement('div');
    div.className = 'fl-typing';
    div.id = 'fl-typing-indicator';
    div.innerHTML = '<span></span><span></span><span></span>';
    messagesDiv.appendChild(div);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
  }

  function hideTyping() {
    var el = document.getElementById('fl-typing-indicator');
    if (el) el.remove();
  }

  // ===== WHATSAPP CONTEXT INJECTION =====
  // Summary is pre-fetched when the agent sends the WA link (see prefetchWaSummary).
  // On click, open WhatsApp synchronously (no async = no popup blocker).
  messagesDiv.addEventListener('click', function(e) {
    var link = e.target.closest('a[href*="wa.me"]');
    if (!link) return;
    e.preventDefault();
    var url = cachedWaSummary
      ? 'https://wa.me/50670183073?text=' + encodeURIComponent(cachedWaSummary)
      : 'https://wa.me/50670183073';
    window.open(url, '_blank', 'noopener');
  });

  function prefetchWaSummary() {
    fetch('/api/v1/chat/summary', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ history: chatHistory.slice(-10) })
    })
    .then(function(res) { return res.json(); })
    .then(function(data) {
      if (data.summary) cachedWaSummary = data.summary;
    })
    .catch(function() { /* silent fail — user still gets the WA link without summary */ });
  }

  // Re-check auth on storage change (login/logout in another tab)
  window.addEventListener('storage', function(e) {
    if (e.key === 'fl_token' && !e.newValue) {
      // User logged out in another tab — no need to hide chat anymore (it's public)
    }
  });
})();
