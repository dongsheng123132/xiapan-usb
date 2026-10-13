#!/usr/bin/env python3
"""Rebuild the frozen PicoClaw distribution; never run this on a customer USB.
Sources, npm packages and Go modules are pinned by checksums/lockfiles.
Removes self-update entry points and fixes attached-process liveness on Windows.
"""
import argparse, hashlib, json, os, pathlib, shutil, subprocess, urllib.request, zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
MANIFEST_PATH = ROOT / "catalog/picoclaw.json"
PINNED = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
VERSION = PINNED["version"]
SOURCE_URL = PINNED["source_url"]
SOURCE_SHA = PINNED["source_sha256"]
DIST = PINNED["distribution"]
CACHE = ROOT / ".cache" / "picoclaw-source"
SOURCE = CACHE / ("picoclaw-" + VERSION)
SLOTS = {"windows/amd64": "windows-x64", "darwin/arm64": "darwin-arm64", "darwin/amd64": "darwin-x64"}

def sha(path):
    with open(path, "rb") as f:
        return hashlib.file_digest(f, "sha256").hexdigest()

def run(args, cwd, env, timeout=1200):
    subprocess.run(args, cwd=cwd, env=env, check=True, timeout=timeout)

def patch(path, old, new):
    text = path.read_text(encoding="utf-8")
    if new and new in text and old not in text:
        return
    if old not in text:
        if not new: return
        raise RuntimeError(f"upstream patch context changed: {path}")
    path.write_text(text.replace(old, new), encoding="utf-8", newline="\n")

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--platform", action="append", choices=list(SLOTS))
    parser.add_argument("--reuse-source", action="store_true", help="Use the already extracted, locally patched source for a resumed build")
    parser.add_argument("--reuse-frontend", action="store_true", help="Reuse the existing embedded frontend for backend-only patch rebuilds")
    args = parser.parse_args()
    if VERSION != "0.3.1" or PINNED.get("updates_enabled") is not False:
        raise RuntimeError("This reviewed patch recipe only supports frozen v0.3.1 with updates disabled")
    CACHE.mkdir(parents=True, exist_ok=True)
    archive = CACHE / ("v" + VERSION + ".zip")
    if not archive.exists():
        with urllib.request.urlopen(SOURCE_URL, timeout=60) as response, archive.open("wb") as target:
            shutil.copyfileobj(response, target)
    if sha(archive) != SOURCE_SHA:
        raise RuntimeError("fixed source checksum mismatch")
    if not args.reuse_source or not SOURCE.exists():
        with zipfile.ZipFile(archive) as z:
            z.extractall(CACHE)
    patch(SOURCE / "cmd/picoclaw/main.go", '\n\t"github.com/sipeed/picoclaw/pkg/updater"', "")
    patch(SOURCE / "cmd/picoclaw/main.go", '\n\t\tupdater.NewUpdateCommand("picoclaw"),', "")
    patch(SOURCE / "web/backend/api/router.go", "h.registerUpdateRoutes(mux)", "// U-PicoClaw distribution: updates are disabled.")
    # Attached Windows processes have no exec.Cmd.Wait goroutine. Upstream's
    # unconditional true prevents Restart from seeing the killed core exit.
    # Reuse its existing OpenProcess/GetExitCodeProcess implementation instead.
    patch(SOURCE / "web/backend/api/gateway.go",
          '\tif runtime.GOOS == "windows" {\n\t\treturn true\n\t}',
          '\tif runtime.GOOS == "windows" {\n\t\treturn ppid.IsProcessRunning(cmd.Process.Pid)\n\t}')
    # API callers may Restart before the first UI status poll. Discover/attach
    # the running external core before deciding which process to stop.
    patch(SOURCE / "web/backend/api/gateway.go",
          'func (h *Handler) RestartGateway() (int, error) {\n\tready, reason, err := h.gatewayStartReady()',
          'func (h *Handler) RestartGateway() (int, error) {\n\t_ = h.gatewayStatusData()\n\tready, reason, err := h.gatewayStartReady()')
    (SOURCE / "pkg/pid/portable_process.go").write_text(
        'package pid\n\n// IsProcessRunning exposes the existing OS liveness probe for the portable launcher.\nfunc IsProcessRunning(pid int) bool { return isProcessRunning(pid) }\n',
        encoding="utf-8", newline="\n")
    # v0.3.1's React frontend has no self-update control; disabling the API also
    # prevents a manually constructed request from replacing launcher/core.
    frontend = SOURCE / "web/frontend"
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    env["GOCACHE"] = str(ROOT / ".cache/go-build-upstream")
    env["GOMODCACHE"] = str(ROOT / ".cache/go-mod-upstream")
    if args.reuse_frontend:
        if not args.reuse_source or not (SOURCE / "web/backend/dist/index.html").is_file():
            raise RuntimeError("--reuse-frontend requires --reuse-source with an existing frontend build")
    else:
        pnpm = shutil.which("pnpm.cmd") or shutil.which("pnpm")
        if not pnpm: raise RuntimeError("pnpm 10.33.0 is required")
        run([pnpm, "install", "--frozen-lockfile"], frontend, env)
        run([pnpm, "run", "build:backend"], frontend, env)
    manifest_path = MANIFEST_PATH
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    for platform in args.platform or SLOTS:
        goos, goarch = platform.split("/")
        env.update(GOOS=goos, GOARCH=goarch)
        ext = ".exe" if goos == "windows" else ""
        destination = ROOT / "runtime/picoclaw" / SLOTS[platform]
        destination.mkdir(parents=True, exist_ok=True)
        built = {}
        for name, package in [("picoclaw", "./cmd/picoclaw"), ("picoclaw-launcher", "./web/backend")]:
            target = destination / (name + ext)
            staging = target.with_suffix(target.suffix + ".new")
            ldflags = "-s -w -X github.com/sipeed/picoclaw/pkg/config.Version=v" + VERSION + "-" + DIST
            if goos == "windows" and name.endswith("launcher"): ldflags += " -H=windowsgui"
            run(["go", "build", "-trimpath", "-tags", "goolm,stdjson", "-ldflags", ldflags, "-o", str(staging), package], SOURCE, env)
            if target.exists():
                backup = ROOT / ".cache/picoclaw-before-patch" / SLOTS[platform] / target.name
                backup.parent.mkdir(parents=True, exist_ok=True)
                if not backup.exists(): shutil.copy2(target, backup)
            os.replace(staging, target)
            built[name] = (target.relative_to(ROOT).as_posix(), sha(target))
        manifest["platforms"][platform] = dict(path=built["picoclaw"][0], sha256=built["picoclaw"][1],
            launcher_path=built["picoclaw-launcher"][0], launcher_sha256=built["picoclaw-launcher"][1])
        manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print("Built and hash-locked", platform, flush=True)
    shutil.copy2(SOURCE / "LICENSE", ROOT / "licenses/PicoClaw-MIT.txt")
    print("Frozen distribution complete. Package only after integration checks.", flush=True)

if __name__ == "__main__": main()
