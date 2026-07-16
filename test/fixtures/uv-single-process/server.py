import http.server
import os


port = int(os.environ["PORT"])
print(f"info uv fixture listening on {port}", flush=True)
print("warning uv fixture stderr ready", file=__import__("sys").stderr, flush=True)
http.server.ThreadingHTTPServer(("127.0.0.1", port), http.server.SimpleHTTPRequestHandler).serve_forever()

