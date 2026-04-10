import importlib
import io
import queue
import unittest

from rayleabot import protocol as protocol_module


class ProtocolStreamErrorTests(unittest.TestCase):
    def setUp(self):
        self.protocol = importlib.reload(protocol_module)

    def test_reader_loop_closes_stream_on_malformed_json(self):
        protocol = self.protocol
        original_stdin = protocol.sys.stdin
        protocol.sys.stdin = io.StringIO('{"protocol_version":"1","type":"event","plugin_id":"helper"\n')
        self.addCleanup(setattr, protocol.sys, "stdin", original_stdin)

        protocol._reader_loop()

        frame = protocol._frame_queue.get_nowait()
        self.assertIsNone(frame)
        with self.assertRaises(queue.Empty):
            protocol._frame_queue.get_nowait()

    def test_reader_loop_rejects_pending_local_actions_on_malformed_json(self):
        protocol = self.protocol
        original_stdin = protocol.sys.stdin
        protocol.sys.stdin = io.StringIO('{"protocol_version":"1","type":"result","request_id":"broken"\n')
        self.addCleanup(setattr, protocol.sys, "stdin", original_stdin)

        response_queue = queue.Queue(maxsize=1)
        protocol._pending_requests["broken"] = response_queue

        protocol._reader_loop()

        error = response_queue.get_nowait()
        self.assertIsInstance(error, protocol.ProtocolError)
        self.assertIn("malformed protocol json", str(error))


if __name__ == "__main__":
    unittest.main()
