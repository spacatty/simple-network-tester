import { ProxyRecord, RequestConfig, RunSummary } from "@/lib/types";

const API = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:8080";

function authHeaders(token: string, json = true): HeadersInit {
  return {
    ...(json ? { "Content-Type": "application/json" } : {}),
    Authorization: `Bearer ${token}`,
  };
}

async function req<T>(path: string, token: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: { ...authHeaders(token), ...(init?.headers ?? {}) },
    cache: "no-store",
  });
  if (!res.ok) {
    const txt = await res.text();
    throw new Error(txt || `Request failed ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export async function startRun(
  config: RequestConfig,
  token: string
): Promise<{ runId: string }> {
  return req("/api/runs/start", token, { method: "POST", body: JSON.stringify(config) });
}

export function streamRun(
  runId: string,
  token: string,
  onData: (data: unknown) => void,
  onError: (error: Error) => void
) {
  const controller = new AbortController();
  void (async () => {
    try {
      const res = await fetch(`${API}/api/runs/stream?runId=${encodeURIComponent(runId)}`, {
        headers: authHeaders(token, false),
        signal: controller.signal,
        cache: "no-store",
      });
      if (!res.ok || !res.body) {
        throw new Error((await res.text()) || `Stream failed ${res.status}`);
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const chunks = buffer.split("\n\n");
        buffer = chunks.pop() ?? "";
        for (const chunk of chunks) {
          const line = chunk
            .split("\n")
            .find((entry) => entry.startsWith("data:"));
          if (!line) continue;
          onData(JSON.parse(line.slice(5).trim()));
        }
      }
    } catch (error) {
      if (!controller.signal.aborted) {
        onError(error instanceof Error ? error : new Error("Stream failed"));
      }
    }
  })();
  return () => controller.abort();
}

export async function uploadFile(
  file: File,
  token: string
): Promise<{ fileId: string; fileName: string }> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(`${API}/api/files/upload`, {
    method: "POST",
    body: fd,
    headers: authHeaders(token, false),
  });
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return res.json() as Promise<{ fileId: string; fileName: string }>;
}

export async function importProxyList(list: string, token: string): Promise<{ imported: number }> {
  return req("/api/proxies/import", token, { method: "POST", body: JSON.stringify({ list }) });
}

export async function listProxies(token: string): Promise<ProxyRecord[]> {
  return req("/api/proxies", token);
}

export async function recheckProxies(token: string): Promise<ProxyRecord[]> {
  return req("/api/proxies/recheck", token, { method: "POST" });
}

export async function deleteInactiveProxies(token: string): Promise<{ deleted: number }> {
  return req("/api/proxies/delete-inactive", token, { method: "POST" });
}

export async function setProxiesEnabled(
  enabled: boolean,
  token: string
): Promise<{ enabled: boolean }> {
  return req("/api/proxies/enabled", token, { method: "POST", body: JSON.stringify({ enabled }) });
}

export async function listReports(token: string): Promise<RunSummary[]> {
  return req("/api/reports", token);
}

export async function downloadReportCsv(runId: string, token: string) {
  const res = await fetch(`${API}/api/reports/export?runId=${encodeURIComponent(runId)}`, {
    headers: authHeaders(token, false),
  });
  if (!res.ok) {
    throw new Error(await res.text());
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `report_${runId}.csv`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
