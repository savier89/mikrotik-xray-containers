#!/usr/bin/env python3
"""
sing-box Management API for MikroTik
=====================================
REST API for managing sing-box container:
- Subscription management (add, remove, list, select)
- Server selection (manual, random, fastest, round-robin)
- Statistics (traffic, connections, latency)
- Config management (view, edit, reload)
- Logs access

Endpoints:
  GET  /api/status           - Container status
  GET  /api/stats            - Traffic statistics
  GET  /api/connections      - Active connections
  GET  /api/subscriptions    - List subscriptions
  POST /api/subscriptions    - Add subscription
  DEL  /api/subscriptions/:id - Remove subscription
  GET  /api/servers          - List servers from active subscription
  POST /api/servers/select   - Select server by index
  POST /api/servers/test     - Test servers, select fastest
  GET  /api/config           - Current sing-box config
  POST /api/config/reload    - Reload sing-box config
  GET  /api/logs             - Container logs
  GET  /api/health           - Health check
"""

import json
import os
import subprocess
import time
import threading
import urllib.request
import urllib.error
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime

# Configuration
API_PORT = int(os.environ.get('API_PORT', '9090'))
SINGBOX_API_ADDR = os.environ.get('SINGBOX_API_ADDR', '127.0.0.1:20123')
SINGBOX_API_TOKEN = os.environ.get('SINGBOX_API_TOKEN', '')
SUB_DIR = '/etc/sing-box/subscriptions'
CONFIG_FILE = '/sing-box.json'
LOG_FILE = '/tmp/sing-box.log'
PID_FILE = '/tmp/.singbox_pid'

# State
subscriptions = []
active_subscription = None
selected_server = None
server_stats = {}
start_time = time.time()

def load_subscriptions():
    """Load subscriptions from disk."""
    global subscriptions
    try:
        with open(f'{SUB_DIR}/subscriptions.json', 'r') as f:
            subscriptions = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        subscriptions = []

def save_subscriptions():
    """Save subscriptions to disk."""
    os.makedirs(SUB_DIR, exist_ok=True)
    with open(f'{SUB_DIR}/subscriptions.json', 'w') as f:
        json.dump(subscriptions, f, indent=2)

def get_singbox_status():
    """Get sing-box process status."""
    try:
        with open(PID_FILE, 'r') as f:
            pid = int(f.read().strip())
        os.kill(pid, 0)
        return {'running': True, 'pid': pid, 'uptime': time.time() - start_time}
    except (FileNotFoundError, ProcessLookupError, ValueError):
        return {'running': False, 'pid': None, 'uptime': 0}

def get_traffic_stats():
    """Get traffic statistics from sing-box Clash API."""
    try:
        url = f'http://{SINGBOX_API_ADDR}/traffic'
        req = urllib.request.Request(url)
        if SINGBOX_API_TOKEN:
            req.add_header('Authorization', f'Bearer {SINGBOX_API_TOKEN}')
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
            return {
                'upload': data.get('up', 0),
                'download': data.get('down', 0),
                'total': data.get('up', 0) + data.get('down', 0)
            }
    except Exception as e:
        return {'error': str(e)}

def get_connections():
    """Get active connections from sing-box Clash API."""
    try:
        url = f'http://{SINGBOX_API_ADDR}/connections'
        req = urllib.request.Request(url)
        if SINGBOX_API_TOKEN:
            req.add_header('Authorization', f'Bearer {SINGBOX_API_TOKEN}')
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
            return data
    except Exception as e:
        return {'error': str(e)}

def fetch_subscription(sub_url, user_agent='curl/8.0.0'):
    """Fetch and decode subscription content."""
    try:
        req = urllib.request.Request(sub_url)
        req.add_header('User-Agent', user_agent)
        with urllib.request.urlopen(req, timeout=15) as resp:
            content = resp.read().decode('utf-8')
        # Try base64 decode
        import base64
        try:
            decoded = base64.b64decode(content).decode('utf-8')
            if '://' in decoded:
                content = decoded
        except:
            pass
        return content
    except Exception as e:
        return None

def parse_servers(content):
    """Parse server links from subscription content."""
    import re
    servers = re.findall(r'(hysteria2|vless|vmess|trojan|ss)://[^[:space:]"<>,]+', content)
    return servers

def test_server_latency(server_url, timeout=5):
    """Test server latency using wget."""
    import re
    match = re.search(r'@([^:]+)', server_url)
    if not match:
        return None
    server = match.group(1)
    start = time.time()
    try:
        subprocess.run(
            ['wget', '-qO-', '--timeout=%s' % timeout, '--tries=1', f'https://{server}'],
            capture_output=True, timeout=timeout+1
        )
        return int((time.time() - start) * 1000)
    except:
        return None

def reload_singbox():
    """Reload sing-box via SIGHUP."""
    try:
        with open(PID_FILE, 'r') as f:
            pid = int(f.read().strip())
        os.kill(pid, 1)  # SIGHUP
        return True
    except:
        return False

class APIHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        """Suppress default logging."""
        pass

    def send_json(self, data, status=200):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(data, indent=2).encode())

    def read_body(self):
        content_length = int(self.headers.get('Content-Length', 0))
        if content_length > 0:
            return json.loads(self.rfile.read(content_length))
        return {}

    def do_GET(self):
        path = self.path.rstrip('/')

        if path == '/api/health':
            self.send_json({'status': 'ok', 'timestamp': datetime.now().isoformat()})

        elif path == '/api/status':
            status = get_singbox_status()
            status['api_port'] = API_PORT
            status['subscriptions_count'] = len(subscriptions)
            status['active_subscription'] = active_subscription
            status['selected_server'] = selected_server
            self.send_json(status)

        elif path == '/api/stats':
            self.send_json(get_traffic_stats())

        elif path == '/api/connections':
            self.send_json(get_connections())

        elif path == '/api/subscriptions':
            self.send_json({'subscriptions': subscriptions, 'active': active_subscription})

        elif path == '/api/servers':
            if not active_subscription:
                self.send_json({'error': 'No active subscription'}, 400)
                return
            sub = next((s for s in subscriptions if s['id'] == active_subscription), None)
            if not sub:
                self.send_json({'error': 'Subscription not found'}, 404)
                return
            servers = sub.get('servers', [])
            self.send_json({
                'subscription': sub['name'],
                'servers': servers,
                'selected': selected_server,
                'stats': server_stats
            })

        elif path == '/api/config':
            try:
                with open(CONFIG_FILE, 'r') as f:
                    config = json.load(f)
                self.send_json(config)
            except Exception as e:
                self.send_json({'error': str(e)}, 500)

        elif path == '/api/logs':
            try:
                with open(LOG_FILE, 'r') as f:
                    logs = f.read()
                self.send_json({'logs': logs})
            except FileNotFoundError:
                self.send_json({'logs': 'Log file not found'})

        else:
            self.send_json({'error': 'Not found'}, 404)

    def do_POST(self):
        global selected_server, active_subscription
        path = self.path.rstrip('/')
        body = self.read_body()

        if path == '/api/subscriptions':
            name = body.get('name', 'Subscription')
            url = body.get('url', '')
            if not url:
                self.send_json({'error': 'URL required'}, 400)
                return

            # Fetch subscription
            content = fetch_subscription(url)
            if not content:
                self.send_json({'error': 'Failed to fetch subscription'}, 500)
                return

            servers = parse_servers(content)
            sub_id = f'sub_{int(time.time())}'
            sub = {
                'id': sub_id,
                'name': name,
                'url': url,
                'servers': servers,
                'created': datetime.now().isoformat(),
                'updated': datetime.now().isoformat(),
                'server_count': len(servers)
            }
            subscriptions.append(sub)
            save_subscriptions()
            self.send_json({'subscription': sub, 'message': 'Subscription added'}, 201)

        elif path == '/api/servers/select':
            index = body.get('index', 0)
            if not active_subscription:
                self.send_json({'error': 'No active subscription'}, 400)
                return
            sub = next((s for s in subscriptions if s['id'] == active_subscription), None)
            if not sub or not sub.get('servers'):
                self.send_json({'error': 'No servers available'}, 400)
                return
            if index >= len(sub['servers']):
                self.send_json({'error': 'Invalid server index'}, 400)
                return
            selected_server = index
            self.send_json({
                'selected': index,
                'server': sub['servers'][index],
                'message': 'Server selected'
            })

        elif path == '/api/servers/test':
            timeout = body.get('timeout', 5)
            if not active_subscription:
                self.send_json({'error': 'No active subscription'}, 400)
                return
            sub = next((s for s in subscriptions if s['id'] == active_subscription), None)
            if not sub or not sub.get('servers'):
                self.send_json({'error': 'No servers available'}, 400)
                return

            results = []
            for i, server in enumerate(sub['servers']):
                latency = test_server_latency(server, timeout)
                results.append({'index': i, 'server': server, 'latency': latency})
                server_stats[f'server_{i}'] = {'latency': latency, 'tested': datetime.now().isoformat()}

            # Find fastest
            valid = [r for r in results if r['latency'] is not None]
            if valid:
                fastest = min(valid, key=lambda x: x['latency'])
                selected_server = fastest['index']
                self.send_json({
                    'results': results,
                    'fastest': fastest,
                    'selected': fastest['index'],
                    'message': f'Server {fastest["index"]} selected ({fastest["latency"]}ms)'
                })
            else:
                self.send_json({
                    'results': results,
                    'error': 'All servers unreachable',
                    'message': 'No server could be reached'
                })

        elif path == '/api/config/reload':
            if reload_singbox():
                self.send_json({'message': 'sing-box reload signal sent'})
            else:
                self.send_json({'error': 'Failed to reload sing-box'}, 500)

        else:
            self.send_json({'error': 'Not found'}, 404)

    def do_DELETE(self):
        global active_subscription
        path = self.path.rstrip('/')

        if path.startswith('/api/subscriptions/'):
            sub_id = path.split('/')[-1]
            if active_subscription == sub_id:
                active_subscription = None
            subscriptions[:] = [s for s in subscriptions if s['id'] != sub_id]
            save_subscriptions()
            self.send_json({'message': f'Subscription {sub_id} removed'})
        else:
            self.send_json({'error': 'Not found'}, 404)

def main():
    global start_time
    start_time = time.time()
    load_subscriptions()

    server = HTTPServer(('0.0.0.0', API_PORT), APIHandler)
    print(f'Management API started on port {API_PORT}')
    print(f'Endpoints:')
    print(f'  GET  /api/health')
    print(f'  GET  /api/status')
    print(f'  GET  /api/stats')
    print(f'  GET  /api/connections')
    print(f'  GET  /api/subscriptions')
    print(f'  POST /api/subscriptions')
    print(f'  DEL  /api/subscriptions/:id')
    print(f'  GET  /api/servers')
    print(f'  POST /api/servers/select')
    print(f'  POST /api/servers/test')
    print(f'  GET  /api/config')
    print(f'  POST /api/config/reload')
    print(f'  GET  /api/logs')
    server.serve_forever()

if __name__ == '__main__':
    main()
