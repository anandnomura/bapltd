    let adminActionPending = false;
    function requestAdminCredential(path, body) {
      return new Promise(resolve => {
        const dialog = document.createElement("dialog");
        dialog.className = "admin-confirm-dialog";
        dialog.innerHTML = `<form method="dialog">
          <h2>Confirm administrative action</h2>
          <p class="action-summary"></p>
          <p>This affects live workloads. Enter the admin credential for this action only.</p>
          <label>Admin credential<input name="credential" type="password" autocomplete="off" required></label>
          <p role="note">BAP currently uses a shared admin token, not an individual password account.</p>
          <div><button value="cancel" formnovalidate>Cancel</button><button value="confirm">Confirm action</button></div>
        </form>`;
        let detail = {};
        try { detail = JSON.parse(body || "{}"); } catch (_) {}
        dialog.querySelector(".action-summary").textContent = path.endsWith("kill-switch")
          ? (detail.enabled ? "Freeze all workloads" : "Restore all workloads")
          : `${detail.action || "Run"}: ${detail.target || path}`;
        dialog.addEventListener("close", () => {
          const input = dialog.querySelector("input");
          const credential = dialog.returnValue === "confirm" ? input.value : null;
          input.value = "";
          dialog.remove();
          resolve(credential);
        }, { once: true });
        document.body.appendChild(dialog);
        dialog.showModal();
        dialog.querySelector("input").focus();
      });
    }
    async function apiFetch(path, options = {}) {
      const method = (options.method || "GET").toUpperCase();
      const administrative = options.admin === true || (method !== "GET" && method !== "HEAD");
      if (administrative && adminActionPending) throw new Error("An administrative action is already pending");
      if (administrative) adminActionPending = true;
      let credential = null;
      try {
        const headers = new Headers(options.headers || {});
        if (administrative) {
          if (location.protocol !== "https:" && !["localhost", "127.0.0.1", "[::1]"].includes(location.hostname)) {
            throw new Error("Use HTTPS before sending an administrative credential to a remote server");
          }
          credential = await requestAdminCredential(path, options.body);
          if (!credential) throw new Error("Action cancelled; no request was sent");
          headers.set("Authorization", `Bearer ${credential}`);
        }
        const response = await fetch(path, { ...options, headers, credentials: "omit", cache: "no-store", redirect: "error", signal: AbortSignal.timeout(10000) });
        if (!response.ok) throw new Error(`Request failed (HTTP ${response.status}). Check the credential, remote-admin setting and connection.`);
        return response;
      } catch (error) {
        if (administrative) window.dispatchEvent(new CustomEvent("bap-api-error", { detail: error.message }));
        throw error;
      } finally {
        credential = null;
        if (administrative) adminActionPending = false;
      }
    }

window.apiFetch = apiFetch;
