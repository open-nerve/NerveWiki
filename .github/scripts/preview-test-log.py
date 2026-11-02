"""Bound the CI console preview; tee keeps the complete test log as an artifact."""

import hashlib
import sys


for number, line in enumerate(sys.stdin.buffer, 1):
    if len(line) <= 1024:
        sys.stdout.buffer.write(line)
    else:
        # Do not split a UTF-8 character in the console preview. The raw bytes,
        # including the complete SQL parameters and plans, remain in test-go.log.
        prefix = line[:1024].decode("utf-8", errors="ignore").encode("utf-8")
        digest = hashlib.sha256(line).hexdigest()
        suffix = (
            f" [full line {number}: {len(line)} bytes, sha256={digest}; "
            "see test-go.log artifact]\n"
        ).encode("ascii")
        sys.stdout.buffer.write(prefix + suffix)
    sys.stdout.buffer.flush()
