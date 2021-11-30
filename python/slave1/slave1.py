#!/usr/bin/env python3
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import time
import random
from threading import Lock


class S(BaseHTTPRequestHandler):
    locker = Lock()

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
        if self.path == '/internal/post':
            dump = {"records": []}
            for i in range(len(self.server.log["records"])):
                if i == self.server.log["records"][i]["offset"]:
                    dump["records"].append(self.server.log["records"][i])
                else:
                    break

            body = json.dumps(dump)
            print(body)
            self._set_headers(content_type='json')
            self.wfile.write(body.encode('utf-8'))
        elif self.path == '/internal/health':
            self._set_headers()
            self.wfile.write("".encode('utf-8'))

    def append_message_to_log(self, message):
        var = self.locker.acquire(blocking=True, timeout=-1)
        print(f"Lock status: {var}")

        # deduplication
        msg_offset = message["offset"]
        existing_offsets = []
        if len(self.server.log["records"]) != 0:
            existing_offsets = [line["offset"]
                                for line in self.server.log["records"]]
        if msg_offset not in existing_offsets:
            self.server.log["records"].append(message)
            print(f"Message '{msg_offset}' has been written to slave1")
        else:
            print("Existing message duplicate")

        # ordering
        self.server.log["records"] = sorted(
            self.server.log["records"], key=lambda d: d['offset'])

        self.locker.release()

    def do_POST(self):
        # <--- Gets the size of data
        content_length = int(self.headers['Content-Length'])
        # <--- Gets the data itself
        post_data = self.rfile.read(content_length)

        print(self.path)

        # appropriate fmt
        loaded_json = json.loads(post_data)

        if self.path == '/internal/post':
            # random delay
            delay = random.randint(2, 10)
            print(f"Random delay on slave1: {delay}")
            time.sleep(delay)

            if 'records' in loaded_json.keys():
                for msg in loaded_json['records']:
                    self.append_message_to_log(msg)
            else:
                self.append_message_to_log(loaded_json)

            self._set_headers()
            self.wfile.write("".encode('utf-8'))

        elif self.path == '/internal/sync':
            last_offset = loaded_json['offset']
            response = {'offsets': []}

            if len(self.server.log["records"]) != 0:
                existing_offsets = [line["offset"]
                                    for line in self.server.log["records"]]
                for i in range(last_offset+1):
                    if i not in existing_offsets:
                        response["offsets"].append(i)
            else:
                response['offsets'] = list(range(last_offset+1))

            data = json.dumps(response)
            self._set_headers(content_type='json')
            self.wfile.write(data.encode('utf-8'))

    def log_message(self, format, *args):
        pass


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
