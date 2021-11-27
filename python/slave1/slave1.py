#!/usr/bin/env python3
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import time
from multiprocessing import Lock


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
        # total order
        dump = {"records": []}
        for i in range(len(self.server.log["records"])):
            if i == self.server.log["records"][i]["Offset"]:
                dump["records"].append(self.server.log["records"][i])
            else:
                break

        body = json.dumps(dump)
        print(body)
        self._set_headers(content_type='json')
        self.wfile.write(body.encode('utf-8'))

    def append_message_to_log(self, message):
        var = Lock().acquire(block=True, timeout=-1)
        print(var)
        # deduplication
        msg_offset = message["Offset"]
        existing_offsets = []
        if len(self.server.log["records"]) != 0:
            existing_offsets = [line["Offset"] for line in self.server.log["records"]]
        print(msg_offset, existing_offsets)
        if msg_offset not in existing_offsets:
            self.server.log["records"].append(message)
        else:
            print("Existing message duplicate")

        # ordering
        self.server.log["records"] = sorted(self.server.log["records"], key=lambda d: d['Offset'])

    def do_POST(self):
        # <--- Gets the size of data
        content_length = int(self.headers['Content-Length'])
        # <--- Gets the data itself
        post_data = self.rfile.read(content_length)

        # appropriate fmt
        loaded_json = json.loads(post_data)
        if type(loaded_json) == type(dict()):
            self.append_message_to_log(loaded_json)
        elif type(loaded_json) == type(list()):
            for msg in loaded_json:
                self.append_message_to_log(msg)
        else:
            self._set_headers(status_code=415)
            self.wfile.write("".encode('utf-8'))
            return

        time.sleep(5)
        self._set_headers()
        self.wfile.write("".encode('utf-8'))


def run(server_class=ThreadingHTTPServer, handler_class=S, port=9001):
    server_address = ('', port)
    httpd = server_class(server_address, handler_class)
    httpd.log = {"records": []}    # variable for log on server
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    httpd.server_close()


if __name__ == '__main__':
    run()
