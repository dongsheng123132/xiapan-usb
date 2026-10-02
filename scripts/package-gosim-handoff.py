"""Assemble submission documents with an already verified review release."""
import hashlib
import json
import shutil
import subprocess
import zipfile
from datetime import datetime
from pathlib import Path
from uuid import uuid4

ROOT = Path(__file__).resolve().parents[1]


def sha256(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def main():
    receipt = json.loads((ROOT / "evidence/review-candidate.json").read_text(encoding="utf-8-sig"))
    release_zip = Path(receipt["archive"]).resolve()
    if not release_zip.is_relative_to((ROOT / "dist").resolve()):
        raise ValueError("Review archive must belong to this workspace's dist directory")
    if sha256(release_zip) != receipt["sha256"]:
        raise ValueError("Review package has changed since verification")
    with zipfile.ZipFile(release_zip) as archive:
        if archive.testzip() is not None:
            raise ValueError("Review ZIP failed CRC verification")

    stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    destination = ROOT / "dist" / f"GOSIM-submit-{stamp}-{uuid4().hex[:4]}"
    public = destination / "原型网页"
    public.mkdir(parents=True, exist_ok=False)
    documents = [
        "豆包工作-GOSIM提交指令.md",
        "GOSIM报名稿-2026-10-02.md",
        "GOSIM演示分镜-2026-10-02.md",
    ]
    for name in documents:
        shutil.copy2(ROOT / "docs" / name, destination / name)
    shutil.copy2(ROOT / "docs/GOSIM原型页.html", public / "index.html")
    shutil.copy2(release_zip, public / release_zip.name)
    shutil.copy2(release_zip.parent / "PicoClaw-demo.mp4", public / "PicoClaw-demo.mp4")
    # Public metadata deliberately contains no local paths or device state.
    release = {
        "version": receipt["version"],
        "source_commit": receipt["source_commit"],
        "archive": release_zip.name,
        "sha256": receipt["sha256"],
        "bytes": receipt["bytes"],
    }
    (public / "release.json").write_text(json.dumps(release, ensure_ascii=False, indent=2), encoding="utf-8")
    application_commit = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True, timeout=10
    ).strip()
    files = [
        {"path": file.relative_to(destination).as_posix(), "bytes": file.stat().st_size, "sha256": sha256(file)}
        for file in sorted(destination.rglob("*")) if file.is_file()
    ]
    manifest = {
        "submission_status": "not_submitted",
        "application_number": None,
        "public_urls": None,
        "application_material_commit": application_commit,
        "prototype_source_commit": receipt["source_commit"],
        "files": files,
    }
    (destination / "材料清单.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    bundle = destination.with_suffix(".zip")
    with zipfile.ZipFile(bundle, "x", zipfile.ZIP_DEFLATED) as archive:
        for file in sorted(destination.rglob("*")):
            if file.is_file():
                archive.write(file, arcname=file.relative_to(destination).as_posix())
    with zipfile.ZipFile(bundle) as archive:
        if archive.testzip() is not None or len(archive.namelist()) != 8:
            raise ValueError("Submission bundle verification failed")
    result = {
        "directory": str(destination),
        "archive": str(bundle),
        "sha256": sha256(bundle),
        "bytes": bundle.stat().st_size,
        "submitted": False,
    }
    (ROOT / "evidence/gosim-handoff.json").write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
