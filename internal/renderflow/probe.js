// Pomelo render probe. Installed before React loads; counts component renders per
// commit via the DevTools hook (React 16.5+), batches to the dev proxy.
(function () {
  if (window.__POM_RENDER_PROBE__) return;
  window.__POM_RENDER_PROBE__ = true;
  var TARGET = document.currentScript && document.currentScript.getAttribute("data-target") || "";
  var ENDPOINT = "/_pom_dev/_pom/render?target=" + encodeURIComponent(TARGET);
  var MAX_RENDERS = 200, MAX_COMMITS = 50, FLUSH_MS = 500;
  var PerformedWork = 1;
  var pending = [], truncated = false, probeMs = 0, timer = null;

  function nameOf(fiber) {
    var t = fiber.type;
    if (!t) return null;
    if (typeof t === "function") return t.displayName || t.name || "Anonymous";
    if (typeof t === "object") {
      if (t.displayName) return t.displayName;
      var inner = t.render || t.type;
      if (inner) return inner.displayName || inner.name || "Anonymous";
    }
    return null;
  }
  function shallowDiff(a, b) {
    if (a === b) return null;
    if (!a || !b) return "all";
    var keys = [], k;
    for (k in b) if (k !== "children" && a[k] !== b[k]) keys.push(k);
    for (k in a) if (!(k in b) && keys.indexOf(k) < 0) keys.push(k);
    return keys.length ? keys.slice(0, 6).join(",") : null;
  }
  function stateChanged(fiber, alt) {
    var s = fiber.memoizedState, p = alt.memoizedState;
    while (s || p) {
      if (!s || !p) return true;
      if (s.memoizedState !== p.memoizedState) return true;
      s = s.next; p = p.next;
    }
    return false;
  }
  function walk(root) {
    var out = [], byName = {};
    var stack = [root.current.child];
    while (stack.length) {
      var f = stack.pop();
      if (!f) continue;
      if (f.sibling) stack.push(f.sibling);
      if (f.child) stack.push(f.child);
      if (!(f.flags & PerformedWork) && !(f.effectTag & PerformedWork)) continue;
      var name = nameOf(f);
      if (!name || typeof f.type === "string") continue;
      var self = f.actualDuration || 0;
      for (var c = f.child; c; c = c.sibling) self -= c.actualDuration || 0;
      if (self < 0) self = 0;
      var why = "mount", wasted = false, alt = f.alternate;
      if (alt) {
        var props = shallowDiff(alt.memoizedProps, f.memoizedProps);
        if (props) why = "props:" + props;
        else if (stateChanged(f, alt)) why = "state";
        else { why = "parent"; wasted = true; }
      }
      var key = name + "|" + why;
      var e = byName[key];
      if (e) { e.count++; e.self += self; continue; }
      if (out.length >= MAX_RENDERS) { truncated = true; continue; }
      e = { name: name, self: Math.round(self * 100) / 100, why: why, wasted: wasted, count: 1 };
      var src = f._debugSource;
      if (src && src.fileName) e.src = { file: src.fileName, line: src.lineNumber || 0 };
      byName[key] = e; out.push(e);
    }
    for (var i = 0; i < out.length; i++) out[i].self = Math.round(out[i].self * 100) / 100;
    return out;
  }
  function onCommit(rendererID, root) {
    var t0 = performance.now();
    try {
      var renders = walk(root);
      if (renders.length) {
        if (pending.length >= MAX_COMMITS) { truncated = true; pending.shift(); }
        pending.push({ t: Date.now(), dur: Math.round((root.current.actualDuration || 0) * 100) / 100, renders: renders });
      }
    } catch (e) { /* never break the app */ }
    probeMs += performance.now() - t0;
    if (!timer) timer = setTimeout(flush, FLUSH_MS);
  }
  function flush() {
    timer = null;
    if (!pending.length) return;
    var body = JSON.stringify({ commits: pending, probe_ms: Math.round(probeMs * 100) / 100, truncated: truncated });
    pending = []; truncated = false; probeMs = 0;
    try {
      if (!(navigator.sendBeacon && navigator.sendBeacon(ENDPOINT, new Blob([body], { type: "application/json" }))))
        fetch(ENDPOINT, { method: "POST", body: body, headers: { "content-type": "application/json" }, keepalive: true }).catch(function () {});
    } catch (e) {}
  }

  var existing = window.__REACT_DEVTOOLS_GLOBAL_HOOK__;
  if (existing && typeof existing.onCommitFiberRoot === "function") {
    var prev = existing.onCommitFiberRoot;
    existing.onCommitFiberRoot = function (id, root, prio, didError) { try { onCommit(id, root); } catch (e) {} return prev.apply(this, arguments); };
    return;
  }
  var renderers = new Map(), nextID = 1;
  window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {
    supportsFiber: true, renderers: renderers,
    inject: function (r) { var id = nextID++; renderers.set(id, r); return id; },
    onCommitFiberRoot: function (id, root) { onCommit(id, root); },
    onCommitFiberUnmount: function () {}, onPostCommitFiberRoot: function () {},
    onScheduleFiberRoot: function () {}, checkDCE: function () {}, emit: function () {}, on: function () {}, off: function () {}, sub: function () { return function () {}; },
    isDisabled: false
  };
})();
