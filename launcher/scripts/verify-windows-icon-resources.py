import argparse
import ctypes
from ctypes import wintypes
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import struct
import sys


ROOT = Path(__file__).resolve().parents[2]
SIZES = [16, 24, 32, 48, 64, 128, 256]


def digest(data):
    return hashlib.sha256(data).hexdigest()


def source_frames(data):
    reserved, kind, count = struct.unpack_from("<HHH", data)
    if (reserved, kind) != (0, 1):
        raise ValueError("Source file is not an ICO")
    frames = {}
    for index in range(count):
        entry = data[6 + index * 16:22 + index * 16]
        width, height, _, _, _, _, length, offset = struct.unpack("<BBBBHHII", entry)
        size = width or 256
        if size != (height or 256) or size in frames or offset + length > len(data):
            raise ValueError("Invalid source ICO frame")
        frames[size] = {"directory": entry[:12], "payload": data[offset:offset + length]}
    if sorted(frames) != SIZES:
        raise ValueError(f"Source ICO sizes differ from {SIZES}")
    return frames


def main():
    parser = argparse.ArgumentParser(description="Compare final EXE icon resources with the launcher source ICO using read-only Windows resource APIs.")
    parser.add_argument("--exe", type=Path, default=ROOT / "launcher/dist/package/win-unpacked/RayleaLauncher.exe")
    parser.add_argument("--ico", type=Path, default=ROOT / "launcher/assets/icon.ico")
    parser.add_argument("--evidence-dir", type=Path)
    args = parser.parse_args()
    if sys.platform != "win32":
        parser.error("Native Windows resource verification requires Windows")

    exe_path = args.exe.resolve()
    ico_path = args.ico.resolve()
    exe_sha = digest(exe_path.read_bytes())
    ico_bytes = ico_path.read_bytes()
    source = source_frames(ico_bytes)
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    pointer = ctypes.c_void_p
    names_callback = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HMODULE, pointer, pointer, ctypes.c_ssize_t)
    languages_callback = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HMODULE, pointer, pointer, wintypes.WORD, ctypes.c_ssize_t)
    signatures = {
        "LoadLibraryExW": ([wintypes.LPCWSTR, wintypes.HANDLE, wintypes.DWORD], wintypes.HMODULE),
        "FreeLibrary": ([wintypes.HMODULE], wintypes.BOOL),
        "EnumResourceNamesW": ([wintypes.HMODULE, pointer, names_callback, ctypes.c_ssize_t], wintypes.BOOL),
        "EnumResourceLanguagesW": ([wintypes.HMODULE, pointer, pointer, languages_callback, ctypes.c_ssize_t], wintypes.BOOL),
        "FindResourceExW": ([wintypes.HMODULE, pointer, pointer, wintypes.WORD], pointer),
        "SizeofResource": ([wintypes.HMODULE, pointer], wintypes.DWORD),
        "LoadResource": ([wintypes.HMODULE, pointer], pointer),
        "LockResource": ([pointer], pointer),
    }
    for name, (arguments, result) in signatures.items():
        function = getattr(kernel, name)
        function.argtypes = arguments
        function.restype = result

    def resource_name(name):
        return pointer(name) if isinstance(name, int) else ctypes.cast(ctypes.c_wchar_p(name), pointer)

    module = kernel.LoadLibraryExW(str(exe_path), None, 0x00000002 | 0x00000020)
    if not module:
        raise ctypes.WinError(ctypes.get_last_error())
    report = {
        "checked_at": datetime.now(timezone.utc).isoformat(),
        "method": "LoadLibraryExW(LOAD_LIBRARY_AS_DATAFILE | LOAD_LIBRARY_AS_IMAGE_RESOURCE), EnumResourceNamesW, EnumResourceLanguagesW, FindResourceExW",
        "exe": str(exe_path),
        "exe_sha256": exe_sha,
        "source_ico": str(ico_path),
        "source_ico_sha256": digest(ico_bytes),
        "expected_sizes": SIZES,
        "groups": [],
        "errors": [],
    }
    extracted = []
    try:
        names = []

        @names_callback
        def collect_name(_module, _kind, name, _context):
            names.append(name if (name or 0) <= 65535 else ctypes.wstring_at(name))
            return True

        if not kernel.EnumResourceNamesW(module, pointer(14), collect_name, 0):
            raise ctypes.WinError(ctypes.get_last_error())

        def read_resource(kind, name, language):
            resource = kernel.FindResourceExW(module, pointer(kind), resource_name(name), language)
            if not resource:
                raise ctypes.WinError(ctypes.get_last_error())
            length = kernel.SizeofResource(module, resource)
            address = kernel.LockResource(kernel.LoadResource(module, resource))
            if not address or not length:
                raise ctypes.WinError(ctypes.get_last_error())
            return ctypes.string_at(address, length)

        for name in names:
            languages = []

            @languages_callback
            def collect_language(_module, _kind, _name, language, _context):
                languages.append(language)
                return True

            if not kernel.EnumResourceLanguagesW(module, pointer(14), resource_name(name), collect_language, 0):
                raise ctypes.WinError(ctypes.get_last_error())
            for language in languages:
                group_data = read_resource(14, name, language)
                reserved, kind, count = struct.unpack_from("<HHH", group_data)
                if (reserved, kind) != (0, 1) or len(group_data) != 6 + count * 14:
                    raise ValueError("Invalid RT_GROUP_ICON data")
                group = {"resource_type": "RT_GROUP_ICON", "id": name, "language": language, "frames": []}
                group_frames = []
                for index in range(count):
                    entry = group_data[6 + index * 14:20 + index * 14]
                    width, height, _, _, _, depth, length, resource_id = struct.unpack("<BBBBHHIH", entry)
                    size = width or 256
                    payload = read_resource(3, resource_id, language)
                    expected = source.get(size)
                    matches = expected is not None and payload == expected["payload"]
                    directory_matches = expected is not None and entry[:12] == expected["directory"]
                    group["frames"].append({
                        "resource_type": "RT_ICON", "id": resource_id, "width": size, "height": height or 256,
                        "bit_depth": depth, "bytes": len(payload), "sha256": digest(payload),
                        "source_sha256": digest(expected["payload"]) if expected else None,
                        "payload_equal": matches, "directory_equal": directory_matches,
                    })
                    if not matches or not directory_matches or length != len(payload):
                        report["errors"].append(f"Group {name}, language {language}, frame {size}px differs from source ICO")
                    group_frames.append((entry[:12], payload))
                if sorted(frame["width"] for frame in group["frames"]) != SIZES:
                    report["errors"].append(f"Group {name}, language {language} has unexpected sizes")
                report["groups"].append(group)
                if not extracted:
                    extracted = group_frames
    finally:
        kernel.FreeLibrary(module)

    if not report["groups"]:
        report["errors"].append("No RT_GROUP_ICON resources found")
    if digest(exe_path.read_bytes()) != exe_sha:
        report["errors"].append("EXE changed during resource verification")
    report["passed"] = not report["errors"]
    if args.evidence_dir:
        args.evidence_dir.mkdir(parents=True, exist_ok=True)
        offset = 6 + 16 * len(extracted)
        directory = []
        payloads = []
        for metadata, payload in extracted:
            directory.append(metadata + struct.pack("<I", offset))
            payloads.append(payload)
            offset += len(payload)
            if metadata[0] == 0 and payload.startswith(b"\x89PNG\r\n\x1a\n"):
                (args.evidence_dir / "launcher-exe-icon-256.png").write_bytes(payload)
        (args.evidence_dir / "launcher-exe-icons.ico").write_bytes(struct.pack("<HHH", 0, 1, len(extracted)) + b"".join(directory + payloads))
        (args.evidence_dir / "native-icon-resources.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    if not report["passed"]:
        print("\n".join(report["errors"]), file=sys.stderr)
        return 1
    print(f"Verified {len(report['groups'])} RT_GROUP_ICON resource(s): {','.join(map(str, SIZES))}px; every RT_ICON payload and directory matches source ICO.")
    print(f"EXE SHA256: {exe_sha}")
    if args.evidence_dir:
        print(f"Evidence: {args.evidence_dir.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
