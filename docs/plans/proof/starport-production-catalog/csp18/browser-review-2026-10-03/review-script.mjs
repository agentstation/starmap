// CSP18 lead browser review of the static documentation site against a local
// gateway build. Drives a real Chromium through Playwright, records measured
// observations, and writes captures plus a measurements file into OUT.
//
// Usage: node csp18-browser-review.mjs <open-gateway-url> <guarded-gateway-url> <out-dir>
import { chromium } from "/private/tmp/open-e2ee-private-sdk-sb7/node_modules/playwright/index.mjs";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const [OPEN, GUARDED, OUT] = process.argv.slice(2);
if (!OPEN || !GUARDED || !OUT) throw new Error("usage: <open-url> <guarded-url> <out-dir>");
mkdirSync(OUT, { recursive: true });

const PAGES = ["", "troubleshoot/recovery/", "search/"];
const TOPIC = "troubleshoot/recovery/";
const results = {}; // observation name -> { pass: boolean, evidence }
const failures = [];
const record = (name, pass, evidence) => {
  results[name] = { pass: Boolean(pass), evidence };
  if (!pass) failures.push(`${name}: ${JSON.stringify(evidence).slice(0, 400)}`);
  console.log(`${pass ? "PASS" : "FAIL"} ${name}`);
};
const url = (path) => `${OPEN}/docs/${path}`;

const browser = await chromium.launch();
const version = browser.version();

async function context(opts = {}) {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 }, colorScheme: "dark", ...opts });
  return ctx;
}

// Rendered text contrast of every visible text node against its painted
// background, resolved by walking up to the first opaque background.
const CONTRAST_SCRIPT = `(() => {
  const parse = (v) => { const m = /rgba?\\((\\d+),\\s*(\\d+),\\s*(\\d+)(?:,\\s*([\\d.]+))?\\)/.exec(v); return m ? [+m[1], +m[2], +m[3], m[4] === undefined ? 1 : +m[4]] : null; };
  const lum = ([r, g, b]) => { const c = (x) => { x /= 255; return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4; }; return 0.2126 * c(r) + 0.7152 * c(g) + 0.0722 * c(b); };
  const over = (t, b) => [0, 1, 2].map((i) => t[i] * t[3] + b[i] * (1 - t[3])).concat(1);
  const ground = (el) => { let acc = null; for (let e = el; e; e = e.parentElement) { const c = parse(getComputedStyle(e).backgroundColor); if (c && c[3] > 0) { acc = acc ? over(acc, c) : c; if (acc[3] >= 1) return acc; } } const canvas = parse(getComputedStyle(document.documentElement).backgroundColor) || [255, 255, 255, 1]; return acc ? over(acc, canvas) : canvas; };
  const samples = [];
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let node;
  while ((node = walker.nextNode())) {
    const text = node.textContent.trim(); if (!text) continue;
    const el = node.parentElement; if (!el || el.closest('.sr-only, script, style, [hidden], template')) continue;
    const cs = getComputedStyle(el); if (cs.visibility === 'hidden' || cs.display === 'none' || cs.opacity === '0') continue;
    const rect = el.getBoundingClientRect(); if (rect.width === 0 || rect.height === 0) continue;
    const fg = parse(cs.color); const bg = ground(el); if (!fg) continue;
    const f = fg[3] < 1 ? over(fg, bg) : fg;
    const [a, b] = [lum(f), lum(bg)].sort((x, y) => y - x);
    samples.push({ text: text.slice(0, 40), tag: el.tagName.toLowerCase(), cls: el.className && typeof el.className === 'string' ? el.className.slice(0, 40) : '', size: parseFloat(cs.fontSize), ratio: Math.round(((a + 0.05) / (b + 0.05)) * 100) / 100 });
  }
  return samples;
})()`;

const OVERFLOW_SCRIPT = `(() => {
  const d = document.documentElement; const w = d.clientWidth;
  const wide = [];
  for (const el of document.querySelectorAll('body *')) {
    const r = el.getBoundingClientRect(); if (r.width === 0) continue;
    if (r.right > w + 1 || r.left < -1) { const cs = getComputedStyle(el); if (cs.position === 'fixed' || el.closest('.sr-only, .skip-link')) continue; let scroller = false; for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) { const ps = getComputedStyle(p); if (['auto', 'scroll'].includes(ps.overflowX) || ['auto', 'scroll'].includes(ps.overflow)) { scroller = true; break; } } if (scroller) continue; wide.push({ tag: el.tagName.toLowerCase(), cls: String(el.className).slice(0, 40), right: Math.round(r.right), left: Math.round(r.left) }); }
  }
  return { clientWidth: w, scrollWidth: d.scrollWidth, bodyScrollWidth: document.body.scrollWidth, wide: wide.slice(0, 10) };
})()`;

// Clipped text: an element with text whose content is wider or taller than
// its box while the box does not scroll.
const CLIP_SCRIPT = `(() => {
  const clipped = [];
  for (const el of document.querySelectorAll('main *, header *, nav *, footer *')) {
    if (!el.childNodes.length || el.closest('.sr-only, [hidden], svg')) continue;
    const cs = getComputedStyle(el);
    if (cs.display === 'none' || cs.visibility === 'hidden') continue;
    const scrolls = ['auto', 'scroll'].includes(cs.overflowX) || ['auto', 'scroll'].includes(cs.overflowY) || ['auto', 'scroll'].includes(cs.overflow);
    if (scrolls) continue;
    if (cs.overflow === 'hidden' || cs.overflowX === 'hidden' || cs.overflowY === 'hidden') {
      if (el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1) clipped.push({ tag: el.tagName.toLowerCase(), cls: String(el.className).slice(0, 40), sw: el.scrollWidth, cw: el.clientWidth, sh: el.scrollHeight, ch: el.clientHeight });
    }
  }
  return clipped.slice(0, 10);
})()`;

const measurements = { browser: `Chromium ${version} (Playwright)`, pages: {} };

// ---- 1. Desktop, both themes, with rendered contrast ----
for (const theme of ["dark", "light"]) {
  const ctx = await context({ colorScheme: theme });
  const page = await ctx.newPage();
  let minRatio = Infinity, minSample = null, count = 0, applied = null, wrongTheme = [];
  for (const path of PAGES) {
    await page.goto(url(path), { waitUntil: "networkidle" });
    applied = await page.evaluate("document.documentElement.dataset.theme || 'dark'");
    if (applied !== theme) wrongTheme.push(path);
    const samples = await page.evaluate(CONTRAST_SCRIPT);
    count += samples.length;
    for (const s of samples) if (s.ratio < minRatio) { minRatio = s.ratio; minSample = { ...s, path }; }
    const name = `${path.replace(/\/$/, "").replace(/\//g, "-") || "home"}-${theme}-1280.png`;
    await page.screenshot({ path: join(OUT, name), fullPage: true });
    measurements.pages[`${path || "home"}:${theme}:1280`] = { samples: samples.length, min: Math.min(...samples.map((s) => s.ratio)) };
  }
  record(`desktop_${theme}`, wrongTheme.length === 0 && count > 0, { pages: PAGES.length, theme_applied: applied, wrong: wrongTheme });
  record(`contrast_${theme}`, minRatio >= 4.5, { samples: count, min_ratio: minRatio, min_sample: minSample });
  // Open version selector and capture the chevron state.
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  await page.locator("details.version-selector summary").click();
  const open = await page.evaluate("document.querySelector('details.version-selector').open");
  const chevron = await page.evaluate("getComputedStyle(document.querySelector('details.version-selector summary'), '::after').transform");
  await page.screenshot({ path: join(OUT, `version-selector-open-${theme}.png`), clip: { x: 0, y: 0, width: 1280, height: 320 } });
  record(`version_selector_${theme}`, open && chevron && chevron !== "none", { open, chevron });
  await ctx.close();
}

// ---- 2. Reflow at 320 CSS px, both themes, no horizontal scroll ----
for (const theme of ["dark", "light"]) {
  const ctx = await context({ viewport: { width: 320, height: 640 }, colorScheme: theme });
  const page = await ctx.newPage();
  const overflow = [];
  for (const path of PAGES) {
    await page.goto(url(path), { waitUntil: "networkidle" });
    const o = await page.evaluate(OVERFLOW_SCRIPT);
    if (o.scrollWidth > 320 || o.bodyScrollWidth > 320 || o.wide.length) overflow.push({ path, ...o });
    const name = `${path.replace(/\/$/, "").replace(/\//g, "-") || "home"}-${theme}-320.png`;
    await page.screenshot({ path: join(OUT, name), fullPage: true });
  }
  record(`reflow_320_${theme}`, overflow.length === 0, { pages: PAGES.length, overflow });
  await ctx.close();
}

// ---- 3. 200 % zoom (640 CSS px layout at device scale 2) and 400 % (320) ----
{
  const ctx = await context({ viewport: { width: 640, height: 450 }, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  const overflow = [];
  for (const path of PAGES) {
    await page.goto(url(path), { waitUntil: "networkidle" });
    const o = await page.evaluate(OVERFLOW_SCRIPT);
    if (o.scrollWidth > 640 || o.wide.length) overflow.push({ path, ...o });
  }
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  await page.screenshot({ path: join(OUT, "recovery-zoom-200.png"), fullPage: false });
  record("zoom_200", overflow.length === 0, { layout_width: 640, device_scale: 2, overflow });
  await ctx.close();
}
{
  const ctx = await context({ viewport: { width: 320, height: 225 }, deviceScaleFactor: 4 });
  const page = await ctx.newPage();
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  const o = await page.evaluate(OVERFLOW_SCRIPT);
  await page.screenshot({ path: join(OUT, "recovery-zoom-400.png"), fullPage: false });
  record("zoom_400", o.scrollWidth <= 320 && o.wide.length === 0, { layout_width: 320, device_scale: 4, ...o });
  await ctx.close();
}

// ---- 4. WCAG 1.4.12 text spacing at 1280 and 320 ----
const SPACING = `* { line-height: 1.5 !important; letter-spacing: 0.12em !important; word-spacing: 0.16em !important; } p { margin-bottom: 2em !important; }`;
for (const width of [1280, 320]) {
  const ctx = await context({ viewport: { width, height: width === 320 ? 640 : 900 } });
  const page = await ctx.newPage();
  const problems = [];
  for (const path of PAGES) {
    await page.goto(url(path), { waitUntil: "networkidle" });
    await page.addStyleTag({ content: SPACING });
    await page.waitForTimeout(100);
    const o = await page.evaluate(OVERFLOW_SCRIPT);
    const clipped = await page.evaluate(CLIP_SCRIPT);
    if (o.scrollWidth > width || o.wide.length || clipped.length) problems.push({ path, overflow: o, clipped });
    if (path === TOPIC) await page.screenshot({ path: join(OUT, `recovery-text-spacing-${width}.png`), fullPage: true });
  }
  record(`text_spacing_${width}`, problems.length === 0, { pages: PAGES.length, problems });
  await ctx.close();
}

// ---- 5. Keyboard: skip link, focus order, visible focus, code scroll ----
{
  const ctx = await context();
  const page = await ctx.newPage();
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  await page.keyboard.press("Tab");
  const skip = await page.evaluate(`(() => { const a = document.activeElement; const cs = getComputedStyle(a); return { cls: a.className, href: a.getAttribute('href'), transform: cs.transform, visible: a.getBoundingClientRect().top >= 0 && a.getBoundingClientRect().height > 0 }; })()`);
  await page.keyboard.press("Enter");
  await page.waitForTimeout(100);
  const afterSkip = await page.evaluate(`(() => ({ active: document.activeElement.id || document.activeElement.tagName, hash: location.hash }))()`);
  record("skip_link", skip.cls === "skip-link" && skip.href === "#content" && skip.transform === "none" && skip.visible && (afterSkip.active === "content" || afterSkip.hash === "#content"), { skip, afterSkip });

  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  const order = [];
  const noOutline = [];
  for (let i = 0; i < 60; i++) {
    await page.keyboard.press("Tab");
    const info = await page.evaluate(`(() => { const a = document.activeElement; if (!a || a === document.body) return null; const cs = getComputedStyle(a); const r = a.getBoundingClientRect(); return { tag: a.tagName.toLowerCase(), name: (a.getAttribute('aria-label') || a.textContent || '').trim().slice(0, 40), role: a.getAttribute('role'), outlineStyle: cs.outlineStyle, outlineWidth: cs.outlineWidth, outlineColor: cs.outlineColor, inView: r.top >= 0 && r.bottom <= innerHeight, h: Math.round(r.height) }; })()`);
    if (!info) break;
    order.push(info);
    if (info.outlineStyle === "none" || parseFloat(info.outlineWidth) < 2) noOutline.push(info);
  }
  const focusOutline = await page.evaluate("getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()");
  record("focus_order_and_outline", order.length >= 10 && noOutline.length === 0 && order[0].tag === "a" && order[0].name.toLowerCase().includes("skip"), { stops: order.length, first: order.slice(0, 6), no_outline: noOutline.slice(0, 5), accent: focusOutline });
  writeFileSync(join(OUT, "focus-order-recovery.json"), JSON.stringify(order, null, 2));
  await ctx.close();
}
{
  const ctx = await context({ viewport: { width: 320, height: 640 } });
  const page = await ctx.newPage();
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  const pre = page.locator("main pre").filter({ has: page.locator("code") }).first();
  const scrollable = await page.evaluate(`(() => { const list = [...document.querySelectorAll('main pre')].map((p) => ({ sw: p.scrollWidth, cw: p.clientWidth, tabindex: p.getAttribute('tabindex'), role: p.getAttribute('role'), label: p.getAttribute('aria-label') })); return list; })()`);
  const target = scrollable.findIndex((p) => p.sw > p.cw);
  let scrolled = null;
  if (target >= 0) {
    const el = page.locator("main pre").nth(target);
    await el.focus();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("End");
    await page.waitForTimeout(150);
    scrolled = await el.evaluate((p) => ({ scrollLeft: p.scrollLeft, focused: document.activeElement === p }));
  }
  record("keyboard_code_scroll", target >= 0 && scrolled && scrolled.focused && scrolled.scrollLeft > 0, { blocks: scrollable.length, overflowing: target, scrolled, sample: scrollable[Math.max(target, 0)] });
  await ctx.close();
}

// ---- 6. Screen-reader semantics: landmarks, names, groups, aria snapshot ----
{
  const ctx = await context();
  const page = await ctx.newPage();
  const problems = [];
  const snapshots = {};
  for (const path of PAGES) {
    await page.goto(url(path), { waitUntil: "networkidle" });
    const s = await page.evaluate(`(() => {
      const outside = (sel) => [...document.querySelectorAll(sel)].filter((e) => !e.closest('article, main'));
      return {
        lang: document.documentElement.lang,
        banner: outside('header, [role=banner]').length, contentinfo: outside('footer, [role=contentinfo]').length,
        main: document.querySelectorAll('main, [role=main]').length, h1: document.querySelectorAll('h1').length,
        navs: [...document.querySelectorAll('nav')].map((n) => n.getAttribute('aria-label')),
        searches: [...document.querySelectorAll('[role=search]')].map((n) => n.getAttribute('aria-label')),
        pres: [...document.querySelectorAll('main pre')].map((p) => [p.getAttribute('role'), p.getAttribute('aria-label'), p.getAttribute('tabindex')]),
        tables: [...document.querySelectorAll('main table')].map((t) => [t.parentElement.getAttribute('role'), t.parentElement.getAttribute('aria-label'), t.parentElement.getAttribute('tabindex')]),
        selector: document.querySelector('details.version-selector summary')?.textContent.trim(),
        current: [...document.querySelectorAll('[aria-current=page]')].map((a) => a.getAttribute('href')),
        anchors: [...document.querySelectorAll('main h2 a.heading-anchor, main h3 a.heading-anchor')].map((a) => a.getAttribute('aria-label')).slice(0, 3),
      };
    })()`);
    const bad = [];
    if (s.lang !== "en") bad.push("lang");
    if (s.banner !== 1 || s.contentinfo !== 1 || s.main !== 1 || s.h1 !== 1) bad.push("landmarks");
    if (!s.navs.includes("Documentation") || s.navs.some((n) => !n)) bad.push("nav names");
    if (!s.searches.length || s.searches.some((n) => !n)) bad.push("search name");
    if (s.pres.some(([r, l, t]) => r !== "group" || !/code example$/i.test(l || "") || t !== "0")) bad.push("code groups");
    if (s.tables.some(([r, l, t]) => r !== "region" || !l || t !== "0")) bad.push("table regions");
    if (!s.selector || !s.selector.startsWith("Documentation version")) bad.push("selector name");
    if (path !== "search/" && s.current.length !== 1) bad.push("aria-current");
    if (bad.length) problems.push({ path, bad, s });
    snapshots[path || "home"] = await page.locator("body").ariaSnapshot();
  }
  writeFileSync(join(OUT, "aria-snapshots.txt"), Object.entries(snapshots).map(([k, v]) => `==== /docs/${k === "home" ? "" : k}\n${v}\n`).join("\n"));
  record("screen_reader_semantics", problems.length === 0, { pages: PAGES.length, problems });
  await ctx.close();
}

// ---- 7. Direct fragment reload, sticky header, history marker ----
{
  const ctx = await context();
  const page = await ctx.newPage();
  const checks = [];
  // Each width starts from a fresh document load of the home page, then a
  // cross-document load of the fragment address, then a reload in place.
  // A reload after a viewport change races Chromium's history scroll
  // restoration; the review record states that limit separately.
  for (const width of [1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const id of ["steps", "coordinated-recovery", "related-settings"]) {
      await page.goto(url(""), { waitUntil: "networkidle" });
      await page.goto(`${url(TOPIC)}#${id}`, { waitUntil: "networkidle" });
      await page.waitForTimeout(200);
      const fresh = await page.evaluate(`(() => { const h = document.getElementById('${id}'); const hd = document.querySelector('header.site-header'); return { top: Math.round(h.getBoundingClientRect().top), headerBottom: Math.round(hd.getBoundingClientRect().bottom) }; })()`);
      await page.reload({ waitUntil: "networkidle" });
      await page.waitForTimeout(200);
      const m = await page.evaluate(`(() => { const h = document.getElementById('${id}'); const hd = document.querySelector('header.site-header'); const r = h.getBoundingClientRect(); const hr = hd.getBoundingClientRect(); const atEnd = Math.ceil(scrollY + innerHeight) >= document.documentElement.scrollHeight - 1; return { top: Math.round(r.top), headerBottom: Math.round(hr.bottom), headerHeight: Math.round(hr.height), sticky: getComputedStyle(hd).position, atEnd, inView: r.top >= hr.bottom - 1 && (r.top < innerHeight / 2 || atEnd) }; })()`);
      checks.push({ width, id, fresh, ...m });
      if (width === 1280 && id === "steps") await page.screenshot({ path: join(OUT, "recovery-fragment-steps-reload-1280.png") });
    }
  }
  record("direct_fragment_reload_desktop", checks.every((c) => c.inView && c.sticky === "sticky" && c.fresh.top >= c.fresh.headerBottom - 1), { checks });

  // Narrow: same fragments without a sticky header.
  await page.setViewportSize({ width: 320, height: 640 });
  const narrow = [];
  for (const id of ["steps", "coordinated-recovery", "related-settings"]) {
    await page.goto(url(""), { waitUntil: "networkidle" });
    await page.goto(`${url(TOPIC)}#${id}`, { waitUntil: "networkidle" });
    await page.waitForTimeout(200);
    const before = await page.evaluate(`Math.round(document.getElementById('${id}').getBoundingClientRect().top)`);
    await page.reload({ waitUntil: "networkidle" });
    await page.waitForTimeout(200);
    narrow.push({ id, before, ...(await page.evaluate(`(() => { const r = document.getElementById('${id}').getBoundingClientRect(); const atEnd = Math.ceil(scrollY + innerHeight) >= document.documentElement.scrollHeight - 1; return { top: Math.round(r.top), atEnd, inView: r.top >= -1 && (r.top < innerHeight / 2 || atEnd) }; })()`)) });
  }
  await page.screenshot({ path: join(OUT, "recovery-fragment-related-settings-reload-320.png") });
  record("direct_fragment_reload_320", narrow.every((c) => c.inView && c.before >= -1), { narrow });

  // Sticky columns at the page end: the footer must not push the navigation
  // or the table of contents under the sticky header, and the first
  // navigation link stays fully visible when it takes focus.
  const columns = [];
  for (const width of [1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto(url(""), { waitUntil: "networkidle" });
    await page.goto(url(TOPIC), { waitUntil: "networkidle" });
    await page.evaluate("window.scrollTo(0, document.documentElement.scrollHeight)");
    await page.waitForTimeout(200);
    const m = await page.evaluate(`(() => { const hd = document.querySelector('header.site-header'); const hb = Math.round(hd.getBoundingClientRect().bottom); const box = (sel) => { const el = document.querySelector(sel); if (!el || getComputedStyle(el).display === 'none') return null; const r = el.getBoundingClientRect(); return { top: Math.round(r.top), bottom: Math.round(r.bottom), sticky: getComputedStyle(el).position }; }; const link = document.querySelector('.site-nav a'); link.focus(); const lr = link.getBoundingClientRect(); const foot = document.querySelector('.site-footer').getBoundingClientRect(); return { headerBottom: hb, nav: box('.site-nav'), toc: box('.toc'), firstLink: { focused: document.activeElement === link, top: Math.round(lr.top), bottom: Math.round(lr.bottom) }, footerTop: Math.round(foot.top), footerHeight: Math.round(foot.height), atEnd: Math.ceil(scrollY + innerHeight) >= document.documentElement.scrollHeight - 1, innerHeight }; })()`);
    columns.push({ width, ...m });
    if (width === 1280) await page.screenshot({ path: join(OUT, "recovery-page-end-sticky-columns-1280.png") });
  }
  const columnOk = (c) => c.atEnd && c.nav && c.nav.sticky === "sticky" && c.nav.top >= c.headerBottom && (!c.toc || c.toc.top >= c.headerBottom) && c.firstLink.focused && c.firstLink.top >= c.headerBottom && c.firstLink.bottom <= c.innerHeight;
  record("sticky_columns_page_end", columns.every(columnOk), { columns });

  // History: TOC links, back, forward, and the current-section marker.
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  const toc = page.locator("nav[data-toc] a");
  const n = await toc.count();
  await toc.nth(1).click();
  await page.waitForTimeout(300);
  const h1 = await page.evaluate("location.hash");
  await toc.nth(2).click();
  await page.waitForTimeout(300);
  const h2 = await page.evaluate("location.hash");
  const marker2 = await page.evaluate("document.querySelector('nav[data-toc] a[aria-current=location]')?.getAttribute('href')");
  await page.goBack();
  await page.waitForTimeout(300);
  const back = await page.evaluate("({ hash: location.hash, marker: document.querySelector('nav[data-toc] a[aria-current=location]')?.getAttribute('href') })");
  await page.goForward();
  await page.waitForTimeout(300);
  const fwd = await page.evaluate("({ hash: location.hash, marker: document.querySelector('nav[data-toc] a[aria-current=location]')?.getAttribute('href') })");
  record("history_marker", n > 2 && h1 && h2 && h1 !== h2 && marker2 === h2 && back.hash === h1 && back.marker === h1 && fwd.hash === h2 && fwd.marker === h2, { toc: n, h1, h2, marker2, back, fwd });

  // Stable version links: the selector names the build and links resolve.
  const links = await page.evaluate(`(() => [...document.querySelectorAll('details.version-selector a')].map((a) => ({ href: a.getAttribute('href'), current: a.getAttribute('aria-current'), text: a.textContent.trim() })))()`);
  const manifest = await (await page.request.get(url("manifest.json"))).json();
  const resolved = [];
  for (const l of links) resolved.push((await page.request.get(new URL(l.href, url(TOPIC)).toString())).status());
  record("stable_version_links", links.length >= 1 && links.some((l) => l.current === "true" && l.text.includes(manifest.starport_release)) && resolved.every((s) => s === 200), { links, resolved, release: manifest.starport_release });
  await ctx.close();
}

// ---- 8. Reduced motion ----
{
  const ctx = await context({ reducedMotion: "reduce" });
  const page = await ctx.newPage();
  await page.goto(url(TOPIC), { waitUntil: "networkidle" });
  const m = await page.evaluate(`({ scroll: getComputedStyle(document.documentElement).scrollBehavior, link: getComputedStyle(document.querySelector('.site-nav a')).transitionDuration, summary: getComputedStyle(document.querySelector('details.version-selector summary'), '::after').transitionDuration })`);
  record("reduced_motion", m.scroll === "auto" && ["0s", "none"].includes(m.link) && ["0s", "none"].includes(m.summary), m);
  await ctx.close();
}

// ---- 9. Static docs without a session: requests and storage ----
{
  const ctx = await context();
  const page = await ctx.newPage();
  const requests = [];
  page.on("request", (r) => requests.push({ url: r.url(), method: r.method() }));
  const responses = [];
  page.on("response", (r) => responses.push({ url: r.url(), status: r.status() }));
  for (const path of PAGES) await page.goto(url(path), { waitUntil: "networkidle" });
  await page.locator("header .site-search input").fill("recovery");
  await page.keyboard.press("Enter");
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(300);
  const hits = await page.locator(".search-results a").count();
  const cookies = await ctx.cookies();
  const storage = await page.evaluate("({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) })");
  const outside = requests.filter((r) => !r.url.startsWith(`${OPEN}/docs/`));
  const api = requests.filter((r) => /\/api\/|\/auth|\/session|\/console\//.test(r.url.replace(OPEN, "")));
  const notOk = responses.filter((r) => r.status >= 400);
  await page.screenshot({ path: join(OUT, "search-results-dark-1280.png"), fullPage: true });
  record("static_docs_without_session", outside.length === 0 && api.length === 0 && notOk.length === 0 && cookies.length === 0 && hits > 0, { requests: requests.length, outside, api, notOk, cookies: cookies.length, storage, search_hits: hits });
  writeFileSync(join(OUT, "requests-open-gateway.json"), JSON.stringify(requests, null, 2));
  await ctx.close();
}

// ---- 10. Guarded gateway: docs open, API routes still guarded ----
{
  const ctx = await context();
  const page = await ctx.newPage();
  const docs = await page.goto(`${GUARDED}/docs/${TOPIC}`, { waitUntil: "networkidle" });
  const docsTitle = await page.title();
  const probes = {};
  for (const path of ["/api/v1/models", "/api/v1/catalog/models", "/api/v1/deployment", "/api/v1/settings"]) {
    const r = await page.request.get(`${GUARDED}${path}`, { maxRedirects: 0, failOnStatusCode: false });
    probes[path] = r.status();
  }
  const guarded = Object.values(probes).every((s) => s === 401 || s === 403 || s === 404);
  const anyGuardedCode = Object.values(probes).some((s) => s === 401 || s === 403);
  record("protected_routes_remain_guarded", docs.status() === 200 && docsTitle.length > 0 && guarded && anyGuardedCode, { docs_status: docs.status(), title: docsTitle, probes });
  await ctx.close();
}

await browser.close();
measurements.results = results;
writeFileSync(join(OUT, "measurements.json"), JSON.stringify(measurements, null, 2));
console.log(`\n${Object.keys(results).length} observations, ${failures.length} failed`);
for (const f of failures) console.log("  " + f);
process.exit(failures.length ? 1 : 0);
