#!/usr/bin/env python3
from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class S(BaseHTTPRequestHandler):
    def _set_headers(self, status_code=200, content_type='html'):
        self.send_response(status_code)
        content = ''
        if content_type == 'html':
            content = 'text/html'
        if content_type == 'json':
            content = 'application/json'
        self.send_header('Content-type', content)
        self.end_headers()

    def do_GET(self):
        body = json.dumps(self.server.log)
        print(body)
        self._set_headers(content_type='json')
        self.wfile.write(body.encode('utf-8'))

    def do_POST(self):
        content_length = int(self.headers['Content-Length'])    # <--- Gets the size of data
        post_data = self.rfile.read(content_length)     # <--- Gets the data itself

        # appropriate fmt
        loaded_json = json.loads(post_data)
        self.server.log["records"].append(loaded_json)

        self._set_headers()
        self.wfile.write("".encode('utf-8'))


def run(server_class=HTTPServer, handler_class=S, address='', port=8080):
    server_address = (address, port)
    httpd = server_class(server_address, handler_class)
    httpd.log = {"records": []}    # variable for log on server
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    httpd.server_close()


if __name__ == '__main__':
    from sys import argv

    if len(argv) == 2:
        run(address=str(argv[0]), port=int(argv[1]))
    else:
        run()
