"""Exercise catalog filtering and pinned native GUIs in a new local sandbox.
No credentials, cloud calls, startup edits or cleanup of user files.
"""
import faulthandler
import hashlib
import json
from pathlib import Path
import runpy
import shutil
import subprocess
import uuid
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parent.parent
helpers = runpy.run_path(str(ROOT / 'scripts/verify-toolpack-usb.py'))
App, observe = helpers['App'], helpers['observe_and_close']
EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'


def main():
    faulthandler.dump_traceback_later(90, repeat=True)
    root = ROOT / '.cache' / ('library-smoke-' + uuid.uuid4().hex[:6])
    root.mkdir()
    (root / 'data').mkdir()
    shutil.copy2(ROOT / 'dist/windows-x64/虾盘.exe', root / '虾盘.exe')
    for tool in json.loads((ROOT / 'catalog/portable-tools.json').read_text(encoding='utf-8')):
        shutil.copytree(ROOT / 'runtime/tool-library/windows-x64' / tool['id'], root / tool['directory'])
    app = App({'root': str(root), 'drive': 'library-smoke'})
    results = {'root': str(root), 'tools': [], 'cloud_requests': 0}
    try:
        catalog = app.run('tools.catalog')
        assert len(catalog['portable_tools']) == 6 and all(t['ready'] for t in catalog['portable_tools'])
        assert len(catalog['library_tools']) == 19 and all(t['ready'] is False for t in catalog['library_tools'])
        results['browser_driver'] = 'Playwright Edge; agent-browser daemon failed on this host'
        print('Catalog passed; opening Edge', flush=True)
        with sync_playwright() as p:
            print('Playwright ready', flush=True)
            browser = p.chromium.launch(executable_path=EDGE, headless=True, timeout=15000)
            try:
                page = browser.new_page(viewport={'width': 1280, 'height': 960})
                errors = []
                page.on('pageerror', lambda e: errors.append(str(e)))
                print('Edge ready', flush=True)
                page.goto(app.base, wait_until='domcontentloaded', timeout=20000)
                print('Page loaded', flush=True)
                page.get_by_role('button', name='工具', exact=True).click()
                expect(page.locator('.tool-row')).to_have_count(35)
                page.screenshot(path=str(ROOT / 'evidence/tool-library-desktop.png'))
                page.get_by_label('筛选准备状态').select_option('portable')
                expect(page.locator('.tool-row')).to_have_count(6)
                page.get_by_label('搜索工具').fill('notepad')
                expect(page.locator('.tool-row')).to_have_count(1)
                with page.expect_response(lambda r: r.url.endswith('/api/run') and
                    r.request.post_data_json.get('action') == 'tools.launch', timeout=75000) as response:
                    page.locator('[data-tool-id="notepadpp"]').get_by_role('button', name='打开 ↗').click()
                launched = response.value.json()
                assert launched['ok'], launched
                window = observe(launched['data']['pid'])
                assert window['visible']
                results['tools'].append({'id': 'notepadpp', 'ui_button': True, 'window': window})
                page.get_by_label('筛选准备状态').select_option('all')
                page.get_by_label('搜索工具').fill('Autoruns')
                expect(page.locator('.tool-row')).to_have_count(1)
                assert page.locator('.tool-row button').count() == 0
                assert 'learn.microsoft.com' in page.locator('.tool-download').get_attribute('href')
                page.get_by_label('搜索工具').fill('没有这款工具xyz')
                expect(page.locator('.tool-empty')).to_be_visible()
                page.get_by_label('搜索工具').fill('')
                page.get_by_label('筛选工具分类').select_option('文件与压缩')
                expect(page.locator('.tool-row')).to_have_count(4)
                page.set_viewport_size({'width': 430, 'height': 900})
                assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                page.screenshot(path=str(ROOT / 'evidence/tool-library-mobile.png'))
                assert not errors, errors
                results['ui'] = {'rows': 35, 'portable_filter': 6, 'category_filter': 4,
                    'download_only_no_launch_button': True, 'empty_search': True,
                    'mobile_no_horizontal_overflow': True, 'page_errors': errors}
            finally:
                browser.close()
        for tool in catalog['portable_tools']:
            if tool['id'] == 'notepadpp':
                continue
            launched = app.run('tools.launch', {'tool_id': tool['id'], 'confirmed': True})
            window = observe(launched['pid'])
            assert window['visible'], tool['id']
            results['tools'].append({'id': tool['id'], 'verified': launched['verified'], 'window': window})
            print(tool['id'] + ': native window passed', flush=True)
        for tool in json.loads((ROOT / 'catalog/portable-tools.json').read_text(encoding='utf-8')):
            for rel, digest in tool['files'].items():
                assert hashlib.sha256((root / tool['directory'] / rel).read_bytes()).hexdigest() == digest, tool['id'] + '/' + rel
        results['immutable_files_unchanged_after_use'] = True
        results['ok'] = True
    finally:
        app.stop()
        (ROOT / 'evidence/tool-library-smoke.json').write_text(json.dumps(results, ensure_ascii=False, indent=2), encoding='utf-8')
    print('Tool library UI and 6 native windows passed.', flush=True)


if __name__ == '__main__':
    main()
