'use strict';
// The serving process supplies these constants without scanning the computer.
const hostPlatform = Object.freeze({
  os: document.querySelector('meta[name="xiapan-platform"]')?.content || '',
  arch: document.querySelector('meta[name="xiapan-arch"]')?.content || '',
  version: document.querySelector('meta[name="xiapan-version"]')?.content || ''
});
const isMac = hostPlatform.os === 'darwin';
function platformUI() {
  const version = document.getElementById('app-version');
  if (version) ui(version, '{version} 便携预览', {version: hostPlatform.version.replace('-portable-preview', '')});
  const shortcut = document.querySelector('#new-chat kbd');
  if (shortcut) shortcut.textContent = isMac ? '⌘ ⇧ N' : 'Ctrl ⇧ N';
  if (!isMac) return;
  for (const node of document.querySelectorAll('[data-platform="windows"]')) node.classList.add('hidden');
  const first = document.getElementById('starter-system');
  first.dataset.prompt = '查看这台 Mac 的系统信息和内存情况';
  ui(first.querySelector('strong'), '查看 Mac 系统信息');
  const network = document.getElementById('starter-network');
  network.dataset.prompt = '帮我检查这台 Mac 的网络接口、DNS 和虾盘云连通性';
  ui(document.getElementById('offline-device-note'), '先查看网络接口与 DNS，再打开无线诊断或系统信息。');
  ui(document.getElementById('offline-device-open'), '查看网络接口 ↗');
  const steps = document.querySelector('#rescue-view .rescue-steps');
  const node = (tag, text) => {const n = document.createElement(tag); ui(n, text); return n;};
  const article = (number, title, text, url, label) => {
    const card = document.createElement('article');
    card.append(node('b', number), node('h3', title), node('p', text));
    if (url) {const link = node('a', label); link.href = url; link.target = '_blank'; link.rel = 'noreferrer'; card.append(link);}
    return card;
  };
  steps.replaceChildren(
    article('01 / macOS', '进入 macOS 恢复', hostPlatform.arch === 'arm64'
      ? 'Apple Silicon：关机后按住电源键，看到启动选项后选择“选项”并继续。'
      : 'Intel Mac：开机后立即按住 Command-R，进入 macOS 恢复。',
      'https://support.apple.com/zh-cn/102518', 'Apple 恢复说明 ↗'),
    article('02 / 磁盘工具', '先备份，再检查磁盘', '在恢复环境中打开磁盘工具。先确认备份和目标磁盘，再按 Apple 说明决定是否运行急救。',
      'https://support.apple.com/zh-cn/102611', 'Apple 磁盘急救说明 ↗'),
    article('03 / 保护数据', '保留原始资料', '本程序提供恢复说明，不会自动抹盘或重装 macOS。恢复输出应存到另一块健康介质。')
  );
  const plan = node('button', '在对话里制定方案 ↗');
  plan.className = 'text-button'; plan.dataset.command = '/救援'; steps.lastElementChild.append(plan);
  ui(document.querySelector('#rescue-view .view-heading p'), 'Mac 使用内建恢复环境；本盘用于系统内运行随身 AI 和维护工具。');
}
