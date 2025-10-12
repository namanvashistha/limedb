import requests
import random
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
import threading

peers = ["http://localhost:7001", "http://localhost:7002", "http://localhost:7003", "http://localhost:7004", "http://localhost:7005"]

# Thread-safe counters
success_count = 0
error_count = 0
lock = threading.Lock()

def set_single_value(i):
    """Set a single key-value pair"""
    global success_count, error_count
    
    base_url = random.choice(peers)
    url = f"{base_url}/api/v1/set"
    payload = {
        "key": f"key_{i}",
        "value": f"value_{i}"
    }
    
    try:
        response = requests.post(url, json=payload, timeout=5)
        
        with lock:
            if response.status_code == 200:
                success_count += 1
            else:
                error_count += 1
                print(f"❌ SET Error for key_{i}: {response.status_code} - {response.text}")
        
        return (i, base_url, response.status_code, "SET")
    
    except Exception as e:
        with lock:
            error_count += 1
            print(f"❌ SET Exception for key_{i}: {str(e)}")
        return (i, base_url, "ERROR", "SET")

def get_single_value(i):
    """Get a single value"""
    global success_count, error_count
    
    base_url = random.choice(peers)
    url = f"{base_url}/api/v1/get/key_{i}"
    
    try:
        response = requests.get(url, timeout=5)
        
        with lock:
            if response.status_code == 200:
                success_count += 1
            else:
                error_count += 1
                print(f"❌ GET Error for key_{i}: {response.status_code} - {response.text}")
        
        return (i, base_url, response.status_code, "GET", response.text if response.status_code == 200 else "")
    
    except Exception as e:
        with lock:
            error_count += 1
            print(f"❌ GET Exception for key_{i}: {str(e)}")
        return (i, base_url, "ERROR", "GET", "")

def set_values_concurrent(total_keys=10_000, max_workers=50):
    """Set values concurrently"""
    global success_count, error_count
    success_count = error_count = 0
    
    print(f"🚀 Starting concurrent SET operations...")
    print(f"📊 Keys: {total_keys}, Workers: {max_workers}, Peers: {len(peers)}")
    
    start_time = time.time()
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # Submit all tasks
        futures = [executor.submit(set_single_value, i) for i in range(total_keys)]
        
        # Process completed tasks
        completed = 0
        for future in as_completed(futures):
            completed += 1
            if completed % 1000 == 0:
                elapsed = time.time() - start_time
                rate = completed / elapsed
                print(f"✅ SET Progress: {completed}/{total_keys} ({completed/total_keys*100:.1f}%) - {rate:.1f} req/sec")
    
    elapsed = time.time() - start_time
    total_rate = total_keys / elapsed
    
    print(f"\n📈 SET Results:")
    print(f"   Total: {total_keys} requests")
    print(f"   Success: {success_count}")
    print(f"   Errors: {error_count}")
    print(f"   Time: {elapsed:.2f}s")
    print(f"   Rate: {total_rate:.1f} requests/second")

def get_values_concurrent(start_key=900, end_key=1100, max_workers=20):
    """Get values concurrently"""
    global success_count, error_count
    success_count = error_count = 0
    
    total_keys = end_key - start_key
    print(f"\n🔍 Starting concurrent GET operations...")
    print(f"📊 Keys: {start_key}-{end_key} ({total_keys} total), Workers: {max_workers}")
    
    start_time = time.time()
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # Submit all tasks
        futures = [executor.submit(get_single_value, i) for i in range(start_key, end_key)]
        
        # Process completed tasks
        completed = 0
        for future in as_completed(futures):
            result = future.result()
            completed += 1
            
            if completed % 50 == 0:
                elapsed = time.time() - start_time
                rate = completed / elapsed
                print(f"✅ GET Progress: {completed}/{total_keys} ({completed/total_keys*100:.1f}%) - {rate:.1f} req/sec")
    
    elapsed = time.time() - start_time
    total_rate = total_keys / elapsed
    
    print(f"\n📈 GET Results:")
    print(f"   Total: {total_keys} requests")
    print(f"   Success: {success_count}")
    print(f"   Errors: {error_count}")
    print(f"   Time: {elapsed:.2f}s")
    print(f"   Rate: {total_rate:.1f} requests/second")

def sequential_set_values():
    """Original sequential implementation for comparison"""
    print(f"🐌 Starting sequential SET operations...")
    start_time = time.time()
    
    for i in range(0, 1000):  # Smaller number for comparison
        base_url = random.choice(peers)
        url = f"{base_url}/api/v1/set"
        payload = {
            "key": f"seq_key_{i}",
            "value": f"seq_value_{i}"
        }
        response = requests.post(url, json=payload)
        if i % 100 == 0:
            print(f"Sequential progress: {i}/1000")
        if response.status_code != 200:
            print(response.text)
    
    elapsed = time.time() - start_time
    rate = 1000 / elapsed
    print(f"📈 Sequential Results: 1000 requests in {elapsed:.2f}s ({rate:.1f} req/sec)")

def test_cluster_health():
    """Test if all nodes are responding"""
    print("🏥 Testing cluster health...")
    for i, peer in enumerate(peers, 1):
        try:
            response = requests.get(f"{peer}/api/v1/cluster/state", timeout=3)
            if response.status_code == 200:
                data = response.json()
                print(f"✅ Node {i} ({peer}): Healthy - Node ID {data.get('nodeId')}")
            else:
                print(f"❌ Node {i} ({peer}): Error {response.status_code}")
        except Exception as e:
            print(f"❌ Node {i} ({peer}): Connection failed - {e}")
    print()

if __name__ == "__main__":
    test_cluster_health()
    
    print("Choose test mode:")
    print("1. Concurrent load test (default)")
    print("2. Sequential test (for comparison)")
    print("3. Both concurrent and sequential")
    
    choice = input("Enter choice (1-3, default=1): ").strip() or "1"
    
    if choice == "1":
        # Concurrent test
        set_values_concurrent(total_keys=50000, max_workers=50)
        get_values_concurrent(start_key=0, end_key=50000, max_workers=20)
        
    elif choice == "2":
        # Sequential test
        sequential_set_values()
        
    elif choice == "3":
        # Both for comparison
        print("\n" + "="*50)
        print("CONCURRENT TEST")
        print("="*50)
        set_values_concurrent(total_keys=1000, max_workers=20)
        
        print("\n" + "="*50)
        print("SEQUENTIAL TEST")
        print("="*50)
        sequential_set_values()
        
    else:
        print("Invalid choice, running concurrent test...")
        set_values_concurrent(total_keys=5000, max_workers=50)
        get_values_concurrent(start_key=1000, end_key=1200, max_workers=20)
