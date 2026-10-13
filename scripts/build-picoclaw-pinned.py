#!/usr/bin/env python3
"""Rebuild the frozen PicoClaw distribution; never run this on a customer USB.
Sources, npm packages and Go modules are pinned by checksums/lockfiles.
Only self-update entry points are removed; the upstream agent/UI stay intact.
"""
import argparse, hashlib, json, os, pathlib, shutil, subprocess, urllib.request, zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
SOURCE_URL = "https://codeload.github.com/sipeed/picoclaw/zip/refs/tags/v0.3.1"
SOURCE_SHA = "22507443b0fdd3835cd461cfe1a15e6565702a55b7dd0aa53bbbb0e0ee96484e"
DIST = "u-pico.1"
CACHE = ROOT / ".cache" / "picoclaw-source"
SOURCE = CACHE / "picoclaw-0.3.1"
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
    args = parser.parse_args()
    CACHE.mkdir(parents=True, exist_ok=True)
    archive = CACHE / "v0.3.1.zip"
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
    # v0.3.1's React frontend has no self-update control; disabling the API also
    # prevents a manually constructed request from replacing launcher/core.
    frontend = SOURCE / "web/frontend"
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    env["GOCACHE"] = str(ROOT / ".cache/go-build-upstream")
    env["GOMODCACHE"] = str(ROOT / ".cache/go-mod-upstream")
    pnpm = shutil.which("pnpm.cmd") or shutil.which("pnpm")
    if not pnpm: raise RuntimeError("pnpm 10.33.0 is required")
    run([pnpm, "install", "--frozen-lockfile"], frontend, env)
    run([pnpm, "run", "build:backend"], frontend, env)
    manifest_path = ROOT / "catalog/picoclaw.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest.update(version="0.3.1", distribution=DIST, source_url=SOURCE_URL, source_sha256=SOURCE_SHA, updates_enabled=False)
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
            ldflags = "-s -w -X github.com/sipeed/picoclaw/pkg/config.Version=v0.3.1-" + DIST
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
