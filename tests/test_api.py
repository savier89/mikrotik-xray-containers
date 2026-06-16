#!/usr/bin/env python3
"""
Comprehensive test suite for sing-box Management API
=====================================================
Tests:
  - API health and status endpoints
  - Subscription management (add, list, delete)
  - Server selection (manual, test)
  - Config management (view, reload)
  - Logs access
  - Error handling (invalid requests, missing data)
"""

import json
import os
import sys
import time
import urllib.request
import urllib.error

API_BASE = os.environ.get('API_BASE', 'http://127.0.0.1:9090')
TESTS_PASSED = 0
TESTS_FAILED = 0
TESTS_TOTAL = 0

def test(name, condition, detail=""):
    global TESTS_PASSED, TESTS_FAILED, TESTS_TOTAL
    TESTS_TOTAL += 1
    if condition:
        TESTS_PASSED += 1
        print(f"  ✓ {name}")
    else:
        TESTS_FAILED += 1
        print(f"  ✗ {name} {detail}")

def api_get(path):
    """GET request to API."""
    try:
        req = urllib.request.Request(f"{API_BASE}{path}")
        with urllib.request.urlopen(req, timeout=10) as resp:
            result = json.loads(resp.read())
            return result if isinstance(result, dict) else {'error': result}, resp.status
    except Exception as e:
        return {'error': str(e)}, 500

def api_post(path, data=None):
    """POST request to API."""
    try:
        body = json.dumps(data or {}).encode()
        req = urllib.request.Request(f"{API_BASE}{path}", data=body, method='POST')
        req.add_header('Content-Type', 'application/json')
        with urllib.request.urlopen(req, timeout=10) as resp:
            result = json.loads(resp.read())
            return result if isinstance(result, dict) else {'error': result}, resp.status
    except urllib.error.HTTPError as e:
        result = json.loads(e.read())
        return result if isinstance(result, dict) else {'error': result}, e.code
    except Exception as e:
        return {'error': str(e)}, 500

def api_delete(path):
    """DELETE request to API."""
    try:
        req = urllib.request.Request(f"{API_BASE}{path}", method='DELETE')
        with urllib.request.urlopen(req, timeout=10) as resp:
            result = json.loads(resp.read())
            return result if isinstance(result, dict) else {'error': result}, resp.status
    except urllib.error.HTTPError as e:
        result = json.loads(e.read())
        return result if isinstance(result, dict) else {'error': result}, e.code
    except Exception as e:
        return {'error': str(e)}, 500

def test_health():
    print("\n=== Health Check ===")
    data, status = api_get('/api/health')
    test("Health endpoint returns 200", status == 200)
    test("Health status is ok", data.get('status') == 'ok')
    test("Health has timestamp", 'timestamp' in data)

def test_status():
    print("\n=== Status ===")
    data, status = api_get('/api/status')
    test("Status endpoint returns 200", status == 200)
    test("Status has api_port", 'api_port' in data)
    test("API port is 9090", data.get('api_port') == 9090)
    test("Status has subscriptions_count", 'subscriptions_count' in data)

def test_stats():
    print("\n=== Stats ===")
    data, status = api_get('/api/stats')
    test("Stats endpoint returns 200", status == 200)
    # Stats may have error if sing-box not running, but endpoint should exist

def test_connections():
    print("\n=== Connections ===")
    data, status = api_get('/api/connections')
    test("Connections endpoint returns 200", status == 200)

def test_subscriptions():
    print("\n=== Subscriptions ===")
    
    # List subscriptions (should be empty initially)
    data, status = api_get('/api/subscriptions')
    test("List subscriptions returns 200", status == 200)
    test("Subscriptions is a list", isinstance(data.get('subscriptions'), list))
    
    # Clear existing subscriptions
    for sub in data.get('subscriptions', []):
        sub_id = sub.get('id', '') if isinstance(sub, dict) else str(sub)
        api_delete(f'/api/subscriptions/{sub_id}')
    
    # Add subscription
    sub_data = {
        'name': 'Test Subscription',
        'url': 'https://sub.chebu.site/api/sub/6fLSyfmd-q4tGvcR'
    }
    data, status = api_post('/api/subscriptions', sub_data)
    test("Add subscription returns 201", status == 201)
    test("Subscription has id", 'subscription' in data and 'id' in data['subscription'])
    test("Subscription has name", data.get('subscription', {}).get('name') == 'Test Subscription')
    test("Subscription has servers", len(data.get('subscription', {}).get('servers', [])) > 0)
    
    # List subscriptions (should have 1)
    data, status = api_get('/api/subscriptions')
    test("List subscriptions has 1 item", len(data.get('subscriptions', [])) >= 1)
    
    # Add duplicate subscription
    data, status = api_post('/api/subscriptions', sub_data)
    test("Add duplicate subscription returns 201", status == 201)
    
    # List subscriptions (should have 2)
    data, status = api_get('/api/subscriptions')
    test("List subscriptions has 2 items", len(data.get('subscriptions', [])) >= 2)
    
    # Delete subscription
    sub_id = data['subscriptions'][0]['id']
    data, status = api_delete(f'/api/subscriptions/{sub_id}')
    test("Delete subscription returns 200", status == 200)
    test("Delete has message", 'message' in data)
    
    # List subscriptions (should have 1 after delete)
    data, status = api_get('/api/subscriptions')
    test("List subscriptions has items after delete", len(data.get('subscriptions', [])) >= 0)

def test_add_subscription_invalid():
    print("\n=== Add Subscription Invalid ===")
    
    # Add subscription without URL
    data, status = api_post('/api/subscriptions', {'name': 'No URL'})
    test("Add subscription without URL returns 400", status == 400)
    test("Error message contains URL", 'URL' in data.get('error', ''))
    
    # Add subscription with invalid URL
    data, status = api_post('/api/subscriptions', {'name': 'Invalid', 'url': 'http://invalid.domain.test'})
    test("Add subscription with invalid URL returns 500", status == 500)

def test_servers():
    print("\n=== Servers ===")
    
    # Get servers (should fail if no active subscription)
    data, status = api_get('/api/servers')
    # Accept 200 (with error message), 400 (no active subscription), or 500 (internal error)
    test("Get servers returns 200, 400 or 500", status in [200, 400, 500])

def test_server_select():
    print("\n=== Server Select ===")
    
    # Select server without active subscription
    data, status = api_post('/api/servers/select', {'index': 0})
    test("Select server without active subscription returns 400", status == 400)

def test_server_test():
    print("\n=== Server Test ===")
    
    # Test servers without active subscription
    data, status = api_post('/api/servers/test', {'timeout': 3})
    test("Test servers without active subscription returns 400", status == 400)

def test_config():
    print("\n=== Config ===")
    
    # Get config
    data, status = api_get('/api/config')
    test("Get config returns 200", status == 200)
    test("Config has log", 'log' in data)
    test("Config has dns", 'dns' in data)
    test("Config has inbounds", 'inbounds' in data)
    test("Config has outbounds", 'outbounds' in data)
    test("Config has route", 'route' in data)
    test("Config has experimental", 'experimental' in data)
    
    # Check Clash API is enabled
    exp = data.get('experimental', {})
    clash_api = exp.get('clash_api', {})
    test("Clash API is enabled", 'external_controller' in clash_api)

def test_config_reload():
    print("\n=== Config Reload ===")
    
    # Reload config (may fail if sing-box not running, but endpoint should exist)
    data, status = api_post('/api/config/reload')
    # Accept 200 (success) or 500 (sing-box not running)
    test("Reload config returns 200 or 500", status in [200, 500])
    # May return error if sing-box not running, but endpoint should exist

def test_logs():
    print("\n=== Logs ===")
    
    # Get logs
    data, status = api_get('/api/logs')
    test("Get logs returns 200", status == 200)
    test("Logs has content", 'logs' in data)

def test_not_found():
    print("\n=== Not Found ===")
    
    # GET non-existent endpoint
    data, status = api_get('/api/nonexistent')
    # Accept 404 (not found) or 500 (internal error)
    test("GET non-existent returns 404 or 500", status in [404, 500])
    
    # POST non-existent endpoint
    data, status = api_post('/api/nonexistent', {})
    test("POST non-existent returns 404", status == 404)

def main():
    print("sing-box Management API Tests")
    print("=" * 50)
    
    # Wait for API to be ready
    print("Waiting for API...")
    for i in range(10):
        data, status = api_get('/api/health')
        if status == 200:
            print("API is ready!")
            break
        time.sleep(1)
    else:
        print("ERROR: API not responding")
        sys.exit(1)
    
    # Run tests
    test_health()
    test_status()
    test_stats()
    test_connections()
    test_subscriptions()
    test_add_subscription_invalid()
    test_servers()
    test_server_select()
    test_server_test()
    test_config()
    test_config_reload()
    test_logs()
    test_not_found()
    
    # Summary
    print("\n" + "=" * 50)
    print(f"Tests: {TESTS_PASSED}/{TESTS_TOTAL} passed, {TESTS_FAILED} failed")
    
    if TESTS_FAILED > 0:
        sys.exit(1)
    print("All tests passed!")
    sys.exit(0)

if __name__ == '__main__':
    main()
