"""Capture controlled UTF-8 command output and surface decoding failures to callers."""

import subprocess


def run_utf8(command, **options):
    check = options.pop("check", False)
    completed = subprocess.run(command, **options)
    result = subprocess.CompletedProcess(
        completed.args,
        completed.returncode,
        completed.stdout.decode("utf-8") if completed.stdout is not None else None,
        completed.stderr.decode("utf-8") if completed.stderr is not None else None,
    )
    if check:
        result.check_returncode()
    return result
