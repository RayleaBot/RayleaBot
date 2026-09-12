import subprocess
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from process_output import run_utf8


class ProcessOutputTests(unittest.TestCase):
    def test_decodes_both_streams_and_preserves_exit_status(self):
        program = "import sys;sys.stdout.buffer.write('中文输出'.encode('utf8'));sys.stderr.buffer.write('失败'.encode('utf8'));sys.exit(7)"
        result = run_utf8([sys.executable, "-c", program], capture_output=True)
        self.assertEqual((result.returncode, result.stdout, result.stderr), (7, "中文输出", "失败"))
        with self.assertRaises(subprocess.CalledProcessError) as failure:
            run_utf8([sys.executable, "-c", program], capture_output=True, check=True)
        self.assertEqual(failure.exception.stderr, "失败")

    def test_bad_encoding_fails_in_the_calling_thread(self):
        for stream in ("stdout", "stderr"):
            with self.subTest(stream=stream), self.assertRaises(UnicodeDecodeError):
                run_utf8([sys.executable, "-c", f"import sys;sys.{stream}.buffer.write(bytes([255]))"], capture_output=True)
