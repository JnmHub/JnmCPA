(function () {
  "use strict";

  var authFilesByName = new Map();
  var enhancementScheduled = false;
  var stylesInjected = false;

  function normalizeURL(input) {
    try {
      if (typeof input === "string") {
        return new URL(input, window.location.href);
      }
      if (input && typeof input.url === "string") {
        return new URL(input.url, window.location.href);
      }
    } catch (_err) {
      return null;
    }
    return null;
  }

  function isAuthFilesURL(input) {
    var url = normalizeURL(input);
    return !!url && /\/v0\/management\/auth-files$/.test(url.pathname);
  }

  function rememberAuthFiles(payload) {
    if (!payload || !Array.isArray(payload.files)) {
      return;
    }
    var next = new Map();
    payload.files.forEach(function (file) {
      if (!file || typeof file !== "object") {
        return;
      }
      var name = typeof file.name === "string" ? file.name.trim() : "";
      if (name) {
        next.set(name, file);
      }
    });
    if (next.size > 0) {
      authFilesByName = next;
      scheduleEnhancement();
    }
  }

  function tryParseAndRemember(text) {
    if (!text || typeof text !== "string") {
      return;
    }
    try {
      rememberAuthFiles(JSON.parse(text));
    } catch (_err) {
      // Ignore non-JSON responses.
    }
  }

  function installFetchHook() {
    if (typeof window.fetch !== "function") {
      return;
    }
    var originalFetch = window.fetch;
    window.fetch = function () {
      var args = Array.prototype.slice.call(arguments);
      return originalFetch.apply(this, args).then(function (response) {
        try {
          if (isAuthFilesURL(args[0])) {
            response.clone().text().then(tryParseAndRemember).catch(function () {});
          }
        } catch (_err) {
          // Ignore enhancer failures so management page keeps working.
        }
        return response;
      });
    };
  }

  function installXHRHook() {
    if (typeof window.XMLHttpRequest !== "function") {
      return;
    }
    var originalOpen = window.XMLHttpRequest.prototype.open;
    window.XMLHttpRequest.prototype.open = function (method, url) {
      this.__managementEnhancerURL = url;
      return originalOpen.apply(this, arguments);
    };

    var originalSend = window.XMLHttpRequest.prototype.send;
    window.XMLHttpRequest.prototype.send = function () {
      this.addEventListener("loadend", function () {
        try {
          if (isAuthFilesURL(this.__managementEnhancerURL)) {
            tryParseAndRemember(this.responseText);
          }
        } catch (_err) {
          // Ignore enhancer failures so management page keeps working.
        }
      });
      return originalSend.apply(this, arguments);
    };
  }

  function injectStyles() {
    if (stylesInjected || !document.head) {
      return;
    }
    stylesInjected = true;
    var style = document.createElement("style");
    style.textContent =
      ".management-enhancer-usage{display:flex;flex-wrap:wrap;gap:8px;margin-top:10px}" +
      ".management-enhancer-pill{display:inline-flex;align-items:center;padding:4px 10px;border-radius:999px;border:1px solid rgba(139,134,128,.24);background:rgba(139,134,128,.08);color:var(--text-secondary,#6b7280);font-size:12px;line-height:1.2;font-weight:600}" +
      ".management-enhancer-pill.is-true{background:rgba(16,185,129,.12);border-color:rgba(16,185,129,.28);color:#047857}" +
      ".management-enhancer-pill.is-false{background:rgba(239,68,68,.1);border-color:rgba(239,68,68,.24);color:#b91c1c}" +
      ".management-enhancer-pill.is-neutral{background:rgba(99,102,241,.08);border-color:rgba(99,102,241,.18);color:#4338ca}";
    document.head.appendChild(style);
  }

  function createOrUpdatePill(container, key, className, text) {
    var pill = container.querySelector("[data-enhancer-pill='" + key + "']");
    if (!pill) {
      pill = document.createElement("span");
      pill.className = "management-enhancer-pill";
      pill.dataset.enhancerPill = key;
      container.appendChild(pill);
    }
    pill.className = "management-enhancer-pill " + className;
    pill.textContent = text;
  }

  function isFutureRetry(value) {
    if (!value) {
      return false;
    }
    var ts = new Date(value).getTime();
    return Number.isFinite(ts) && ts > Date.now();
  }

  function resolveUsable(file) {
    if (file && typeof file.usable === "boolean") {
      return file.usable;
    }
    if (!file) {
      return false;
    }
    return !(file.disabled || isFutureRetry(file.next_retry_after));
  }

  function resolveCooling(file) {
    if (file && typeof file.cooling === "boolean") {
      return file.cooling;
    }
    if (!file) {
      return false;
    }
    return isFutureRetry(file.next_retry_after);
  }

  function formatRetryValue(value) {
    if (!value) {
      return "null";
    }
    var date = new Date(value);
    if (!Number.isFinite(date.getTime())) {
      return String(value);
    }
    return date.toLocaleString("zh-CN", { hour12: false });
  }

  function attachUsageSummary(card, file) {
    var cardMain = card.querySelector("[class*='AuthFilesPage-module__fileCardMain___']");
    if (!cardMain) {
      return;
    }

    var anchor =
      cardMain.querySelector("[class*='AuthFilesPage-module__cardInsights___']") ||
      cardMain.querySelector("[class*='AuthFilesPage-module__cardActions___']");
    var summary = cardMain.querySelector("[data-auth-usage-summary]");
    if (!summary) {
      summary = document.createElement("div");
      summary.className = "management-enhancer-usage";
      summary.dataset.authUsageSummary = "true";
      if (anchor && anchor.parentNode === cardMain) {
        cardMain.insertBefore(summary, anchor);
      } else {
        cardMain.appendChild(summary);
      }
    }

    var usable = resolveUsable(file);
    var cooling = resolveCooling(file);
    var retryAt = file ? file.next_retry_after : null;

    createOrUpdatePill(summary, "usable", usable ? "is-true" : "is-false", "usable: " + String(usable));
    createOrUpdatePill(summary, "cooling", cooling ? "is-true" : "is-false", "cooling: " + String(cooling));
    createOrUpdatePill(summary, "next-retry-after", "is-neutral", "next_retry_after: " + formatRetryValue(retryAt));

    card.dataset.authUsable = String(usable);
    card.dataset.authCooling = String(cooling);
    card.dataset.authNextRetryAfter = retryAt == null ? "null" : String(retryAt);
  }

  function enhanceCards() {
    if (!authFilesByName.size) {
      return;
    }
    injectStyles();
    var cards = document.querySelectorAll("[class*='AuthFilesPage-module__fileCard___']");
    cards.forEach(function (card) {
      var nameEl = card.querySelector("[class*='AuthFilesPage-module__fileName___']");
      var name = nameEl && typeof nameEl.textContent === "string" ? nameEl.textContent.trim() : "";
      if (!name) {
        return;
      }
      var file = authFilesByName.get(name);
      if (!file) {
        return;
      }
      attachUsageSummary(card, file);
    });
  }

  function scheduleEnhancement() {
    if (enhancementScheduled) {
      return;
    }
    enhancementScheduled = true;
    window.requestAnimationFrame(function () {
      enhancementScheduled = false;
      enhanceCards();
    });
  }

  function installObserver() {
    var observer = new MutationObserver(function () {
      scheduleEnhancement();
    });
    observer.observe(document.documentElement, { childList: true, subtree: true });
  }

  function start() {
    installFetchHook();
    installXHRHook();
    injectStyles();
    installObserver();
    scheduleEnhancement();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  } else {
    start();
  }
})();
