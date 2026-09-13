import time
import json
import urllib.request
import subprocess
import os
import sys

def run_50k_benchmark():
    port = 58999
    base_url = f"http://127.0.0.1:{port}"
    server_bin = os.path.abspath("bap-controlplane/bapcontrolplane.exe")
    
    print(f"===============================================================")
    print(f"       BAP Control Plane: 50,000 Event Performance Test (PT)    ")
    print(f"===============================================================")
    print(f"[*] Starting bapcontrolplane on port {port}...")
    
    proc = subprocess.Popen(
        [server_bin, "-port", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL
    )
    
    try:
        # 1. Health check poll
        for _ in range(30):
            try:
                with urllib.request.urlopen(f"{base_url}/api/v1/health", timeout=1) as resp:
                    if resp.status == 200:
                        break
            except Exception:
                time.sleep(0.1)
        else:
            raise RuntimeError("Control plane failed to start")
        print("[+] Control plane is online and ready for load.")
        
        # 2. Start a test session
        sess_req = urllib.request.Request(
            f"{base_url}/api/v1/sessions/start",
            data=json.dumps({"session_id": "sess-loadtest-50k", "app_id": "perf-harness"}).encode("utf-8"),
            headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(sess_req) as resp:
            pass
        
        # 3. Generate and stream 50,000 events in batches of 2,500
        total_events = 50_000
        batch_size = 2_500
        num_batches = total_events // batch_size
        
        print(f"[*] Dispatching {total_events:,} events in {num_batches} batches of {batch_size:,}...")
        
        t0 = time.perf_counter()
        total_ingested = 0
        batch_latencies = []
        
        for b in range(num_batches):
            batch = []
            now_iso = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
            for i in range(batch_size):
                idx = b * batch_size + i
                batch.append({
                    "event_id": f"perf-{idx:06d}",
                    "session_id": "sess-loadtest-50k",
                    "timestamp": now_iso,
                    "source": "claude-code",
                    "executable": "git" if idx % 2 == 0 else "curl",
                    "full_command": "git status" if idx % 2 == 0 else "curl https://internal.corp",
                    "decision": "allow" if idx % 2 == 0 else "deny",
                    "reason": "" if idx % 2 == 0 else "Blocked by policy",
                    "exit_code": 0 if idx % 2 == 0 else 1
                })
            
            data = json.dumps(batch).encode("utf-8")
            b_start = time.perf_counter()
            req = urllib.request.Request(
                f"{base_url}/api/v1/audit/ingest",
                data=data,
                headers={"Content-Type": "application/json"}
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                res = json.loads(resp.read().decode("utf-8"))
                total_ingested += res.get("ingested", 0)
            b_lat = (time.perf_counter() - b_start) * 1000
            batch_latencies.append(b_lat)
            print(f"    Batch {b+1:2d}/{num_batches}: {batch_size:,} events in {b_lat:6.1f}ms ({batch_size / (b_lat/1000):,.0f} ev/s)")
            
        t_total = time.perf_counter() - t0
        throughput = total_events / t_total
        avg_batch_lat = sum(batch_latencies) / len(batch_latencies)
        
        print(f"---------------------------------------------------------------")
        print(f"[+] Successfully ingested: {total_ingested:,} / {total_events:,} events")
        print(f"[+] Total Ingestion Time : {t_total:.2f}s")
        print(f"[+] Overall Throughput   : {throughput:,.0f} events/second")
        print(f"[+] Average Batch Latency: {avg_batch_lat:.1f}ms per {batch_size} events")
        
        # 4. Verify Cryptographic SHA-256 Hash Chain Integrity across all 50K
        print(f"[*] Verifying SHA-256 hash chain over 50,000 sequential events...")
        t_chain0 = time.perf_counter()
        with urllib.request.urlopen(f"{base_url}/api/v1/audit/events") as resp:
            ev_data = json.loads(resp.read().decode("utf-8"))
            chain_status = ev_data.get("chain_status")
        t_chain = (time.perf_counter() - t_chain0) * 1000
        print(f"[+] SHA-256 Chain Status : {chain_status.upper()} (Verified in {t_chain:.1f}ms)")
        
        # 5. Verify Session Aggregates
        with urllib.request.urlopen(f"{base_url}/api/v1/sessions/sess-loadtest-50k") as resp:
            sess_data = json.loads(resp.read().decode("utf-8"))
        print(f"[+] Session Total Events : {sess_data.get('total_events'):,}")
        print(f"[+] Session Allowed Count: {sess_data.get('allowed_count'):,}")
        print(f"[+] Session Denied Count : {sess_data.get('denied_count'):,}")
        print(f"===============================================================")
        
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()

if __name__ == "__main__":
    run_50k_benchmark()
