from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import sys
import tempfile
import urllib.request
import zipfile

PYTHON_VERSION = "3.13.15"
EMBED_SHA256 = "d1f04d990aee1253d8569e8e5104e30fa9f5fa830899f14843448872d936a2cf"
EMBED_URL = f"https://www.python.org/ftp/python/{PYTHON_VERSION}/python-{PYTHON_VERSION}-embed-amd64.zip"
ROOT = Path(__file__).resolve().parent.parent


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def build(output: Path) -> None:
    if sys.platform != "win32" or sys.maxsize <= 2**32:
        raise RuntimeError("The timetable runtime must be assembled on Windows x64")
    lock = ROOT / "scripts" / "timetable-requirements.lock"
    if not lock.is_file():
        raise RuntimeError("Missing committed timetable-requirements.lock")
    output = output.resolve()
    if output.exists() and not (output / "runtime-manifest.json").is_file():
        raise RuntimeError(f"Refusing to replace an unmanaged directory: {output}")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="timetable-build-", dir=output.parent) as temporary:
        work = Path(temporary)
        archive = work / "python.zip"
        request = urllib.request.Request(EMBED_URL, headers={"User-Agent": "Gotack-build"})
        with urllib.request.urlopen(request, timeout=120) as response, archive.open("wb") as target:
            copied = 0
            for chunk in iter(lambda: response.read(1024 * 1024), b""):
                copied += len(chunk)
                if copied > 64 * 1024 * 1024:
                    raise RuntimeError("Python archive exceeds the expected size limit")
                target.write(chunk)
        if sha256(archive) != EMBED_SHA256:
            raise RuntimeError("Python embeddable archive SHA-256 mismatch")
        runtime = work / "runtime"
        runtime.mkdir()
        with zipfile.ZipFile(archive) as source:
            for entry in source.infolist():
                name = PurePosixPath(entry.filename)
                if name.is_absolute() or ".." in name.parts or "\\" in entry.filename or ":" in entry.filename:
                    raise RuntimeError(f"Unsafe Python archive entry: {entry.filename}")
            source.extractall(runtime)
        site_packages = runtime / "Lib" / "site-packages"
        subprocess.run(
            [sys.executable, "-m", "pip", "install", "--disable-pip-version-check", "--no-compile",
             "--only-binary=:all:", "--require-hashes", "--target", str(site_packages), "-r", str(lock)],
            check=True,
        )
        (runtime / "python313._pth").write_text("python313.zip\n.\nLib/site-packages\nimport site\n", encoding="utf-8")
        shutil.copy2(lock, runtime / "requirements.lock")
        smoke = work / "smoke.py"
        smoke.write_text(
            "import json, pathlib, sys, tempfile\n"
            "import defusedxml, openpyxl, ortools\n"
            "from ortools.sat.python import cp_model\n"
            "model = cp_model.CpModel()\n"
            "a = model.new_bool_var('a')\n"
            "b = model.new_bool_var('b')\n"
            "model.add(a + b == 1)\n"
            "model.maximize(a)\n"
            "solver = cp_model.CpSolver()\n"
            "solver.parameters.max_time_in_seconds = 10\n"
            "assert solver.solve(model) == cp_model.OPTIMAL\n"
            "assert solver.value(a) == 1 and solver.value(b) == 0\n"
            "for path in sys.argv[1:]:\n"
            "    book = openpyxl.load_workbook(path)\n"
            "    assert book.sheetnames\n"
            "    with tempfile.TemporaryDirectory() as directory:\n"
            "        output = pathlib.Path(directory) / 'roundtrip.xlsx'\n"
            "        book.save(output)\n"
            "        check = openpyxl.load_workbook(output)\n"
            "        assert check.sheetnames == book.sheetnames\n"
            "        check.close()\n"
            "    book.close()\n"
            "print(json.dumps({'python': sys.version.split()[0], 'ortools': ortools.__version__, 'openpyxl': openpyxl.__version__, 'defusedxml': defusedxml.__version__}))\n",
            encoding="utf-8",
        )
        templates = ROOT / "resources" / "skills" / "timetable" / "assets"
        result = subprocess.run(
            # -B keeps the smoke run from writing bytecode caches into the
            # runtime we are about to ship.
            [str(runtime / "python.exe"), "-I", "-B", str(smoke),
             str(templates / "mau-thoi-khoa-bieu.xlsx"), str(templates / "phan-cong-chuan-hoa.xlsx")],
            check=True, capture_output=True, text=True, timeout=120,
            env={key: value for key, value in os.environ.items() if key.upper() not in {"PYTHONHOME", "PYTHONPATH"}},
        )
        versions = json.loads(result.stdout)
        if versions["python"] != PYTHON_VERSION:
            raise RuntimeError("Packaged Python version mismatch")
        # CPython stamps bytecode caches with the absolute source path, so any
        # __pycache__ left behind would make the shipped runtime carry the
        # builder's filesystem layout and differ between build machines. The
        # runtime ships sources only and recompiles on first use.
        for cache in runtime.rglob("__pycache__"):
            if cache.is_dir():
                shutil.rmtree(cache)
        manifest = {
            "schema": 1,
            "versions": versions,
            "python_archive_sha256": EMBED_SHA256,
            "requirements_sha256": sha256(lock),
            "checks": {"cp_sat": True, "excel_templates_roundtrip": True},
            "files": {
                path.relative_to(runtime).as_posix(): sha256(path)
                for path in sorted(runtime.rglob("*")) if path.is_file()
            },
        }
        (runtime / "runtime-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        previous = work / "previous"
        if output.exists():
            output.rename(previous)
        try:
            runtime.rename(output)
        except OSError:
            if previous.exists():
                previous.rename(output)
            raise
        print(json.dumps({"output": str(output), "versions": versions, "validated": True}))


def main() -> None:
    parser = argparse.ArgumentParser(description="Build Gotack's offline Windows timetable runtime")
    parser.add_argument("--output", type=Path, default=ROOT / "build" / "bin" / "resources" / "python")
    args = parser.parse_args()
    build(args.output)


if __name__ == "__main__":
    main()
