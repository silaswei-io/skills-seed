#!/usr/bin/env python3
"""校验官方发行包后转换为统一 ZIP，供各平台 Go 二进制内嵌。"""
import hashlib
import io
import pathlib
import sys
import tarfile
import urllib.request
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
VERSION = '1.6.2'
PLATFORMS = {'darwin': 'darwin', 'linux': 'linux', 'windows': 'win32'}
ARCHES = {'amd64': 'x64', 'arm64': 'arm64'}


def main():
    system, arch = sys.argv[1:]
    platform = PLATFORMS[system]
    suffix = '.zip' if system == 'windows' else '.tar.gz'
    name = f'codegraph-{platform}-{ARCHES[arch]}{suffix}'
    base = f'https://github.com/colbymchenry/codegraph/releases/download/v{VERSION}/'
    checksums = urllib.request.urlopen(base + 'SHA256SUMS', timeout=60).read().decode()
    expected = next(line.split()[0] for line in checksums.splitlines() if line.split()[-1].lstrip('*') == name)
    data = urllib.request.urlopen(base + name, timeout=120).read()
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError('CodeGraph checksum mismatch')
    entries = []
    if suffix == '.zip':
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            for entry in archive.infolist():
                if not entry.is_dir():
                    entries.append((entry.filename, archive.read(entry), entry.external_attr >> 16))
    else:
        with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
            for entry in archive.getmembers():
                if entry.isfile():
                    entries.append((entry.name, archive.extractfile(entry).read(), entry.mode))
    # 官方包可能有顶层目录；以 Node 的同级目录作为运行组件根目录。
    executable = 'node.exe' if system == 'windows' else 'node'
    node_path = next(pathlib.PurePosixPath(name) for name, _, _ in entries if pathlib.PurePosixPath(name).name == executable)
    prefix = node_path.parent
    destination = ROOT / 'internal/codegraph/assets/runtime.zip'
    with zipfile.ZipFile(destination, 'w', zipfile.ZIP_DEFLATED) as output:
        for name, content, mode in sorted(entries):
            relative = pathlib.PurePosixPath(name).relative_to(prefix)
            if '..' in relative.parts or relative.is_absolute():
                raise ValueError('Unsafe archive path')
            info = zipfile.ZipInfo(str(relative))
            info.external_attr = (mode & 0o777) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            output.writestr(info, content)
    print(f'Bundled CodeGraph {VERSION}: {destination}')


if __name__ == '__main__':
    main()
