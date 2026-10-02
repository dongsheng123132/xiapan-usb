"""Run the portable app from already prepared, newly created USB test directories."""
import json
import os
from pathlib import Path
import re
import subprocess
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
TARGETS = json.loads((ROOT / "evidence/usb-targets.json").read_text(encoding="utf-8-sig"))
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def request(base, path, action=None, inputs=None, token=None):
    body = None if action is None else json.dumps({"action": action, "input": inputs or {}}).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["X-Xiapan-Token"] = token
    req = urllib.request.Request(base + path, data=body, headers=headers)
    with HTTP.open(req, timeout=65) as response:
        content = response.read().decode()
    return json.loads(content) if path.startswith("/api/") else content

def run(base, token, action, inputs=None):
    r = request(base, "/api/run", action, inputs, token)
    if not r["ok"]:
        raise RuntimeError(r.get("error", "action failed"))
    return r["data"]

results = []
carry_report = None
for i, target in enumerate(TARGETS):
    root = Path(target["root"])
    if not root.is_dir() or not (root / "虾盘.exe").is_file():
        raise RuntimeError("Prepared test directory missing")
    if carry_report:
        dest = root / carry_report[0]
        dest.parent.mkdir(parents=True, exist_ok=True)
        if dest.exists():
            raise RuntimeError("Refusing to overwrite a report")
        dest.write_bytes(carry_report[1])
    base = f"http://127.0.0.1:{18796+i}"
    log_path = ROOT / "evidence" / f"app-{target['drive']}.log"
    with log_path.open("w", encoding="utf-8") as log:
        env = os.environ.copy()
        proc = subprocess.Popen([str(root / "虾盘.exe"), "serve", "--port", str(18796+i), "--no-open"], cwd=root, stdout=log, stderr=log, env=env, creationflags=subprocess.CREATE_NO_WINDOW)
        try:
            for _ in range(80):
                try:
                    page = request(base, "/")
                    break
                except Exception:
                    if proc.poll() is not None:
                        raise RuntimeError("Portable app exited: " + log_path.read_text())
                    time.sleep(0.25)
            else:
                raise RuntimeError("Portable app startup timed out")
            token = re.search(r'name="xiapan-token" content="([a-f0-9]+)"', page).group(1)
            system = run(base, token, "system.inspect")
            assert Path(system["portable_root"]).resolve() == root.resolve()
            network = run(base, token, "network.inspect")
            resources = run(base, token, "resources.scan")
            assert any(f["name"].endswith(".gguf") for f in resources["files"])
            rejection = request(base, "/api/run", "tools.install", {"package_id": "portable-check"}, token)
            assert not rejection["ok"]
            installed = run(base, token, "tools.install", {"package_id": "portable-check", "confirmed": True})
            assert installed["verified"]
            report = run(base, token, "report.create")
            reports = run(base, token, "reports.list")
            assert len(reports) >= (2 if carry_report else 1)
            plan = run(base, token, "maintenance.plan", {"message": "格式化 F 盘"})
            assert plan["mode"] == "guide" and plan["suggested_action"] == "resources.scan"
            print(f"{target['drive']}: real inspection, offline tool install and report passed", flush=True)
            start = time.perf_counter()
            model = run(base, token, "models.start", {"confirmed": True})
            startup_seconds = round(time.perf_counter()-start, 3)
            state = run(base, token, "models.inspect")
            selected = next(s for s in state["services"] if s["kind"] == "bundled")
            answer = run(base, token, "maintenance.plan", {"message": "电脑有点慢，请用三条简短中文建议告诉我应该先检测什么。不要声称已经执行了操作。", "backend": "bundled", "model": selected["model"]})
            assert answer["mode"] == "local-model" and answer["reply"].strip()
            assert any('\u4e00' <= c <= '\u9fff' for c in answer["reply"])
            stopped = run(base, token, "models.stop")
            assert stopped["stopped"]
            result = {**target, "system": system, "installation": installed, "resource_count": len(resources["files"]), "report_count": len(reports), "report_path": report["path"], "report_relocation": bool(carry_report), "model": model, "startup_seconds": startup_seconds, "answer": answer, "network_test_condition": "Internet was not disconnected; only loopback model API and local files used. No cloud model API used.", "same_computer": True}
            results.append(result)
            carry_report = (report["path"], (root / report["path"]).read_bytes())
            print(f"{target['drive']}: CPU local model reply passed; startup {startup_seconds}s; answer {answer.get('metrics')}", flush=True)
            (ROOT / "evidence/usb-test-results.json").write_text(json.dumps(results, ensure_ascii=False, indent=2), encoding="utf-8")
        finally:
            try:
                req = urllib.request.Request(base+"/api/quit", data=b"", headers={"X-Xiapan-Token": token})
                HTTP.open(req, timeout=10).close()
            except Exception:
                proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()

print("Both USB drives passed on this Windows computer. This is not a cross-computer or boot-media test.", flush=True)
