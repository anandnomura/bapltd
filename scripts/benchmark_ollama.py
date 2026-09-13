"""
Ollama Governed Inference Benchmark
Demonstrates safe local LLM inference under BAP edge supervision.
"""
import sys
import time
import json
import urllib.request
import urllib.error

OLLAMA_URL = "http://localhost:11434/v1/chat/completions"
MODEL = "claude-3-5-sonnet-20241022:latest"

def main():
    print(f"[*] Connecting to local Ollama ({OLLAMA_URL})...")
    print(f"[*] Targeting model: {MODEL}")

    prompt = "Say the word READY and nothing else"
    payload = {
        "model": MODEL,
        "messages": [
            {"role": "user", "content": prompt}
        ],
        "temperature": 0.1
    }

    t0 = time.perf_counter()
    req = urllib.request.Request(
        OLLAMA_URL,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"}
    )

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            elapsed_ms = (time.perf_counter() - t0) * 1000
            content = data.get("choices", [{}])[0].get("message", {}).get("content", "").strip()
            print("\n" + "=" * 60)
            print("  OLLAMA INFERENCE SUCCESS")
            print("=" * 60)
            print(f"[+] Model Response : {content}")
            print(f"[+] Round-Trip Time: {elapsed_ms:.1f} ms ({elapsed_ms/1000:.2f} s)")
            print("=" * 60 + "\n")
            return 0
    except urllib.error.URLError as e:
        print(f"[!] Could not connect to Ollama on port 11434: {e.reason}")
        print("    Ensure Ollama is running: ollama serve")
        return 1

if __name__ == "__main__":
    sys.exit(main())

