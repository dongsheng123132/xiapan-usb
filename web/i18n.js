'use strict';
// Only application-owned labels are bound. Chat text, JSON evidence, paths and
// editable values are deliberately excluded; no document-wide mutation observer.
//
// The dictionary lives at /locales.json (a top-level file) rather than
// /locales/en.json, because main.go embeds `web/*` non-recursively and a
// subdirectory would never reach the binary.
const I18n = (() => {
  let language = 'zh-CN', dictionary = {}, preferences = null, loaded = false;
  const bindings = new Map();
  const normalize = value => String(value ?? '').trim();
  function text(source, values = {}) {
    const key = normalize(source);
    if (language !== 'en') return key.replace(/\{(\w+)\}/g, (match, key) => Object.hasOwn(values, key) ? String(values[key]) : match);
    // A leading completion tick is a status glyph, not part of the label.
    const tick = key.startsWith('✓ ') ? '✓ ' : '';
    const body = tick ? key.slice(2) : key;
    const translated = dictionary[key] ?? (dictionary[body] ? tick + dictionary[body] : key);
    return translated.replace(/\{(\w+)\}/g, (match, key) => Object.hasOwn(values, key) ? String(values[key]) : match);
  }
  function bind(node, source, values = {}, attribute = '') {
    if (!node) return node;
    if (source && typeof source === 'object') ({source, values} = source);
    const key = attribute || '#text';
    if (!bindings.has(node)) bindings.set(node, new Map());
    bindings.get(node).set(key, {source: normalize(source), values});
    const value = text(source, values);
    if (attribute) node.setAttribute(attribute, value);
    else node.textContent = value;
    return node;
  }
  function staticLabels() {
    const walk = document.createTreeWalker(document.documentElement, NodeFilter.SHOW_TEXT);
    const nodes = [];
    while (walk.nextNode()) {
      const node = walk.currentNode;
      if (/[\u3400-\u9fff]/.test(node.textContent) && !node.parentElement.closest('script,style,pre,code,[data-no-i18n]')) nodes.push(node);
    }
    for (const node of nodes) bind(node, node.textContent);
    // data-command and data-prompt carry values sent to the backend verbatim,
    // so only genuinely presentational attributes are translated.
    for (const node of document.querySelectorAll('[placeholder],[aria-label],[title]')) {
      for (const attr of ['placeholder', 'aria-label', 'title']) {
        if (/[\u3400-\u9fff]/.test(node.getAttribute(attr) || '')) bind(node, node.getAttribute(attr), {}, attr);
      }
    }
  }
  function apply(next) {
    language = next === 'en' ? 'en' : 'zh-CN';
    document.documentElement.lang = language;
    for (const [node, fields] of bindings) {
      if (!node.isConnected) { bindings.delete(node); continue; }
      for (const [attr, spec] of fields) {
        const value = text(spec.source, spec.values);
        if (attr === '#text') node.textContent = value;
        else node.setAttribute(attr, value);
      }
    }
    const select = document.getElementById('language-select');
    if (select) select.value = language;
    document.dispatchEvent(new CustomEvent('languagechange', {detail: {language}}));
  }
  async function init() {
    // A missing or malformed dictionary must never take the whole interface
    // down: fall back to the Chinese source strings and keep running.
    try {
      const response = await fetch('/locales.json');
      if (response.ok) { dictionary = await response.json(); loaded = true; }
      else console.warn('locales.json unavailable; staying in Chinese.');
    } catch (error) {
      console.warn('locales.json could not be loaded; staying in Chinese.', error);
    }
    staticLabels();
    let preferred = /^zh\b/i.test(navigator.language) ? 'zh-CN' : 'en';
    if (!loaded) preferred = 'zh-CN';
    try { preferences = await run('settings.ui.get'); } catch (error) { console.warn('UI preferences unavailable.', error); }
    if (preferences && preferences.language && preferences.language !== 'auto') preferred = preferences.language;
    apply(preferred);
    const select = document.getElementById('language-select');
    if (!select) return;
    select.onchange = async () => {
      const next = select.value;
      select.disabled = true;
      try {
        preferences = await run('settings.ui.save', {expected_revision: preferences.revision, language: next});
        apply(next);
        toast('语言已保存到本盘');
      } catch (error) {
        select.value = language;
        if (typeof fail === 'function') fail(error); else console.warn(error);
      } finally { select.disabled = false; }
    };
  }
  return {text, bind, init, get language() {return language;}, get loaded() {return loaded;}};
})();
const t = (source, values) => I18n.text(source, values);
const ui = (node, source, values) => I18n.bind(node, source, values);
const attr = (node, attribute, source) => I18n.bind(node, source, {}, attribute);
const msg = (source, values = {}) => ({source, values});
