#!/usr/bin/env python3
"""把发行目录打成跨平台可解的 zip。

为什么需要它：Windows 上用资源管理器「发送到压缩文件夹」或旧版
PowerShell `Compress-Archive` 打出来的 zip，条目名会带反斜杠
（`U-PicoClaw\\app\\runtime\\...`）。这种包在 Windows 资源管理器里能解，
但在 macOS / Linux 的 unzip、Python zipfile、7z 里会报
`Illegal byte sequence` 或直接把反斜杠当成文件名字符，等于发不出去。
2026-11-02 的 0.5.8 Windows 包就踩过这个坑。

本脚本保证：
  * 条目分隔符统一为 `/`（zip 规范要求）
  * 非 ASCII 文件名写 UTF-8 标志位（Windows 10+ 资源管理器可正确显示中文）
  * 拒绝绝对路径、`..`、盘符、反斜杠
  * 可选 --prefix 给所有条目加一层根目录（发行包习惯）

用法：
    python3 scripts/make-release-zip.py <源目录> <输出.zip> [--prefix 虾盘-u-claw-lite-0.5.8-windows-x64]

只读源目录，不修改任何输入文件。
"""
import argparse
import os
import pathlib
import sys
import zipfile

CHUNK = 1 << 20


def norm(name: str) -> str:
    return name.replace(os.sep, "/").replace("\\", "/").lstrip("./")


def check(entry: str) -> None:
    if entry != norm(entry):
        raise SystemExit(f"非法条目（含反斜杠或前导 ./）：{entry!r}")
    if entry.startswith("/") or ":" in entry:
        raise SystemExit(f"非法条目（绝对路径/盘符）：{entry!r}")
    if any(part == ".." for part in entry.split("/")):
        raise SystemExit(f"非法条目（含 ..）：{entry!r}")


def main() -> int:
    parser = argparse.ArgumentParser(description="打包发行 zip（跨平台可解）")
    parser.add_argument("source", help="源目录")
    parser.add_argument("output", help="输出 zip 路径（已存在则拒绝）")
    parser.add_argument("--prefix", default="", help="可选的根目录前缀")
    parser.add_argument("--comment", default="", help="zip 注释")
    args = parser.parse_args()

    source = pathlib.Path(args.source).resolve()
    output = pathlib.Path(args.output).resolve()
    if not source.is_dir():
        raise SystemExit(f"源目录不存在：{source}")
    if output.exists():
        raise SystemExit(f"输出已存在，拒绝覆盖：{output}")

    written = 0
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
        if args.comment:
            zf.comment = args.comment.encode("utf-8")
        for root, dirs, files in os.walk(source):
            dirs.sort()
            for name in sorted(files):
                path = pathlib.Path(root) / name
                if path.is_symlink() and not path.exists():
                    raise SystemExit(f"悬空符号链接：{path}")
                rel = path.relative_to(source).as_posix()
                entry = f"{args.prefix.strip('/')}/{rel}" if args.prefix else rel
                check(entry)
                info = zipfile.ZipInfo(entry, date_time=_mtime(path))
                info.create_system = 3  # Unix，这样解压端才会还原可执行位
                info.compress_type = zipfile.ZIP_DEFLATED
                info.external_attr = (path.stat().st_mode & 0o777) << 16
                with open(path, "rb") as src, zf.open(info, "w", force_zip64=True) as dst:
                    while True:
                        chunk = src.read(CHUNK)
                        if not chunk:
                            break
                        dst.write(chunk)
                written += 1
            for name in sorted(dirs):
                path = pathlib.Path(root) / name
                if not any(path.iterdir()):
                    rel = path.relative_to(source).as_posix() + "/"
                    entry = f"{args.prefix.strip('/')}/{rel}" if args.prefix else rel
                    check(entry)
                    zf.writestr(zipfile.ZipInfo(entry, date_time=_mtime(path)), b"")

    size = output.stat().st_size
    sys.stdout.write(f"{output}\n{written} files, {size} bytes ({size / 1048576:.2f} MiB)\n")
    return 0


def _mtime(path: pathlib.Path):
    ts = path.stat().st_mtime
    import datetime

    dt = datetime.datetime.fromtimestamp(ts)
    return (dt.year, dt.month, dt.day, dt.hour, dt.minute, dt.second)


if __name__ == "__main__":
    raise SystemExit(main())
