/**
 * Traccia tracking script. Works as a plain <script> tag (reads its own
 * config from data-* attributes) and as the entry point of the npm package
 * — same file, two distribution paths, zero build step either way.
 *
 * <script src="https://your-traccia.com/t.js" data-project="<project_id>" defer></script>
 * data-host is optional; it defaults to the script's own origin, which is
 * the common case when Traccia is self-hosted on the same domain as its
 * tracking script.
 */
(function (window, document) {
  "use strict";

  var COOKIE_NAME = "_traccia_vid";
  var COOKIE_MAX_AGE = 60 * 60 * 24 * 365 * 2; // 2 years

  function currentScriptConfig() {
    var script = document.currentScript;
    if (!script) return null;
    return {
      projectId: script.getAttribute("data-project"),
      host: script.getAttribute("data-host") || new URL(script.src).origin,
    };
  }

  function generateId() {
    if (window.crypto && window.crypto.randomUUID) {
      return window.crypto.randomUUID();
    }
    // Fallback for older browsers without crypto.randomUUID — not
    // cryptographically strong, but a visitor_id only needs to be unique,
    // not unguessable.
    return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, function (c) {
      var r = (Math.random() * 16) | 0;
      var v = c === "x" ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  function readCookie(name) {
    var match = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"));
    return match ? decodeURIComponent(match[1]) : null;
  }

  function writeCookie(name, value, maxAgeSeconds) {
    document.cookie =
      name + "=" + encodeURIComponent(value) + "; path=/; max-age=" + maxAgeSeconds + "; SameSite=Lax";
  }

  function getOrCreateVisitorId() {
    var id = readCookie(COOKIE_NAME);
    if (!id) {
      id = generateId();
      writeCookie(COOKIE_NAME, id, COOKIE_MAX_AGE);
    }
    return id;
  }

  function Traccia(config) {
    this.projectId = config.projectId;
    this.host = config.host.replace(/\/$/, "");
    this.visitorId = getOrCreateVisitorId();
  }

  Traccia.prototype._send = function (path, body) {
    var url = this.host + path;
    var payload = JSON.stringify(body);
    // keepalive lets the request survive a page unload — the same problem
    // navigator.sendBeacon solves, but fetch(keepalive) also gives us a
    // JSON content-type and a normal request/response if we ever need it.
    if (window.fetch) {
      fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: payload,
        keepalive: true,
      }).catch(function () {});
    } else if (navigator.sendBeacon) {
      navigator.sendBeacon(url, new Blob([payload], { type: "application/json" }));
    }
  };

  Traccia.prototype.track = function (name, metadata) {
    this._send("/api/v1/track", {
      project_id: this.projectId,
      visitor_id: this.visitorId,
      type: name ? "custom" : "pageview",
      name: name || "",
      path: window.location.pathname,
      referrer: document.referrer,
      metadata: metadata || {},
    });
  };

  Traccia.prototype.trackError = function (name, metadata) {
    this._send("/api/v1/track", {
      project_id: this.projectId,
      visitor_id: this.visitorId,
      type: "error",
      name: name,
      path: window.location.pathname,
      referrer: document.referrer,
      metadata: metadata || {},
    });
  };

  Traccia.prototype.identify = function (properties, name) {
    this._send("/api/v1/identify", {
      project_id: this.projectId,
      visitor_id: this.visitorId,
      name: name || "",
      properties: properties || {},
    });
  };

  Traccia.prototype._trackPageview = function () {
    this.track();
  };

  Traccia.prototype._bindAutoEvents = function () {
    var self = this;
    document.addEventListener("click", function (evt) {
      var el = evt.target.closest && evt.target.closest("[data-traccia-event]");
      if (el) self.track(el.getAttribute("data-traccia-event"));
    });
  };

  Traccia.prototype._trackSpaNavigation = function () {
    var self = this;
    var wrap = function (fn) {
      return function () {
        var result = fn.apply(this, arguments);
        self._trackPageview();
        return result;
      };
    };
    history.pushState = wrap(history.pushState);
    history.replaceState = wrap(history.replaceState);
    window.addEventListener("popstate", function () {
      self._trackPageview();
    });
  };

  function init() {
    var config = currentScriptConfig();
    if (!config || !config.projectId) {
      console.warn("traccia: missing data-project attribute, tracking disabled");
      return;
    }

    var instance = new Traccia(config);
    instance._bindAutoEvents();
    instance._trackSpaNavigation();
    instance._trackPageview();
    window.traccia = instance;
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})(window, document);
