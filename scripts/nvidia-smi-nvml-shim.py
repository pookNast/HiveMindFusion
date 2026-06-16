#!/usr/bin/env python3
# nvidia-smi shim — queries NVML via ctypes when the nvidia-smi binary is absent.
# Produces the exact CSV the hivemind Go code expects:
#   nvml.go:20        --query-gpu=memory.used,memory.free,memory.total
#   health.go:243     --query-gpu=memory.used,memory.total
# Both use --format=csv,noheader,nounits.
#
# ponytail: shim because BatKave's nvidia-utils package is broken (transitional
# empty package, binary missing from filesystem despite dpkg claiming installed).
# Upgrade: remove this shim when a real nvidia-smi binary lands in /usr/bin.

import sys
import ctypes


class Memory(ctypes.Structure):
    _fields_ = [
        ("total", ctypes.c_ulonglong),
        ("free", ctypes.c_ulonglong),
        ("used", ctypes.c_ulonglong),
    ]


_FIELD_MAP = {
    "memory.total": "total",
    "memory.free": "free",
    "memory.used": "used",
}


def main():
    query_fields = []
    for arg in sys.argv[1:]:
        if arg.startswith("--query-gpu="):
            query_fields = arg.split("=", 1)[1].split(",")
            break

    if not query_fields:
        sys.stderr.write("nvidia-smi shim: only --query-gpu=memory.* supported\n")
        return 1

    attrs = []
    for f in query_fields:
        attr = _FIELD_MAP.get(f.strip())
        if attr is None:
            sys.stderr.write(f"nvidia-smi shim: unsupported field '{f}'\n")
            return 1
        attrs.append(attr)

    try:
        nvml = ctypes.CDLL("libnvidia-ml.so.1")
    except OSError as e:
        sys.stderr.write(f"nvidia-smi shim: cannot load libnvidia-ml.so.1: {e}\n")
        return 1

    if nvml.nvmlInit_v2() != 0:
        sys.stderr.write("nvidia-smi shim: nvmlInit_v2 failed\n")
        return 1

    count = ctypes.c_uint(0)
    nvml.nvmlDeviceGetCount_v2(ctypes.byref(count))

    for i in range(count.value):
        handle = ctypes.c_void_p()
        if nvml.nvmlDeviceGetHandleByIndex_v2(i, ctypes.byref(handle)) != 0:
            continue
        mem = Memory()
        nvml.nvmlDeviceGetMemoryInfo(handle, ctypes.byref(mem))
        # NVML reports bytes; nvidia-smi reports MiB.
        vals = [str(getattr(mem, attr) // (1024 * 1024)) for attr in attrs]
        print(", ".join(vals))

    nvml.nvmlShutdown()
    return 0


if __name__ == "__main__":
    sys.exit(main())
