from http.server import BaseHTTPRequestHandler, HTTPServer
from sys import argv
import logging
import time
import random
import json


class SecondaryRHandler(BaseHTTPRequestHandler):
    def _set_response(self):
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()

    def do_GET(self):
        body = json.dumps(self.server.log)
        self._set_response()
        self.wfile.write(body.encode('utf-8'))

    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        loaded_json = json.loads(post_data)
        self.server.log["records"].append(loaded_json)
        self._set_response()
        time.sleep(random.randrange(0, 5))  # рандомний
        self.wfile.write("".encode('utf-8'))


def run(server_class=HTTPServer, handler_class=SecondaryRHandler, port=9001):
    logging.basicConfig(level=logging.INFO)
    server_address = ('', port)
    httpd = server_class(server_address, handler_class)
    httpd.log = {"records": []}
    logging.info('Starting httpd...\n')
    try:
        print('Started http server')
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    httpd.server_close()
    logging.info('Stopping httpd...\n')


if __name__ == '__main__':
    if len(argv) == 2:
        run(port=int(argv[0]))
    else:
        run()
