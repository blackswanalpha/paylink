"""Tiny stdlib echo upstream for gateway tests — returns method/path/headers as JSON on any path.

Used as a stand-in for paylink-service / payment-orchestrator so the gateway's behavior (routing,
header injection/stripping, credential hygiene) is directly observable. ECHO_NAME tags which
upstream answered so routing can be asserted.
"""

import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer

NAME = os.environ.get("ECHO_NAME", "echo")
PORT = int(os.environ.get("ECHO_PORT", "8080"))


class Handler(BaseHTTPRequestHandler):
    def _respond(self) -> None:
        body = json.dumps(
            {
                "service": NAME,
                "method": self.command,
                "path": self.path,
                "headers": {k.lower(): v for k, v in self.headers.items()},
            }
        ).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def do_GET(self) -> None:
        self._respond()

    def do_POST(self) -> None:
        self._respond()

    def do_PUT(self) -> None:
        self._respond()

    def do_PATCH(self) -> None:
        self._respond()

    def do_DELETE(self) -> None:
        self._respond()

    def do_HEAD(self) -> None:
        self._respond()

    def log_message(self, *args: object) -> None:  # silence access logs
        pass


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
