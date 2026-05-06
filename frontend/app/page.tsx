"use client";

import { type ReactNode, useMemo, useState } from "react";
import {
  deleteInactiveProxies,
  downloadReportCsv,
  importProxyList,
  listProxies,
  listReports,
  recheckProxies,
  setProxiesEnabled,
  startRun,
  streamRun,
  uploadFile,
} from "@/lib/api";
import { ProxyRecord, RequestConfig, RunSample, RunSummary } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  PieChart,
  Pie,
  Cell,
} from "recharts";

const chartColors = ["#6366f1", "#22c55e", "#ef4444", "#f59e0b", "#06b6d4", "#ec4899"];

type MultipartRow = {
  id: string;
  enabled: boolean;
  kind: "text" | "file";
  fieldName: string;
  value: string;
  fileId: string;
  fileName: string;
};

const defaultConfig: RequestConfig = {
  name: "Quick Test",
  method: "GET",
  url: "http://localhost:3000/api/health",
  headers: {},
  body: {
    type: "none",
    json: "{}",
    form: {},
    multipartFields: {},
    multipartFiles: [],
  },
  successStatuses: [200],
  timeoutMs: 5000,
  concurrency: 5,
  requests: 100,
  durationSeconds: 0,
  rateLimitRps: 0,
  userAgent: {
    mode: "default_rotate",
    fixedValue: "",
    customList: [],
  },
  proxy: { enabled: false },
};

export default function Home() {
  const [tab, setTab] = useState<"builder" | "live" | "proxies" | "reports">("builder");
  const [authToken, setAuthToken] = useState(() =>
    typeof window === "undefined" ? "" : sessionStorage.getItem("loadtester_auth_token") ?? ""
  );
  const [config, setConfig] = useState<RequestConfig>(defaultConfig);
  const [headersText, setHeadersText] = useState("");
  const [formBodyText, setFormBodyText] = useState("");
  const [multipartRows, setMultipartRows] = useState<MultipartRow[]>([
    {
      id: crypto.randomUUID(),
      enabled: true,
      kind: "file",
      fieldName: "file",
      value: "",
      fileId: "",
      fileName: "",
    },
  ]);
  const [successStatusesText, setSuccessStatusesText] = useState("200");
  const [customUaText, setCustomUaText] = useState("");
  const [proxyText, setProxyText] = useState("");

  const [runId, setRunId] = useState("");
  const [samples, setSamples] = useState<RunSample[]>([]);
  const [latestSummary, setLatestSummary] = useState<RunSummary | null>(null);
  const [running, setRunning] = useState(false);
  const [backendError, setBackendError] = useState("");

  const [proxies, setProxies] = useState<ProxyRecord[]>([]);
  const safeProxies = Array.isArray(proxies) ? proxies : [];
  const [proxiesEnabled, setProxiesEnabledState] = useState(false);
  const [reports, setReports] = useState<RunSummary[]>([]);

  function saveAuthToken(value: string) {
    setAuthToken(value);
    if (value) {
      sessionStorage.setItem("loadtester_auth_token", value);
    } else {
      sessionStorage.removeItem("loadtester_auth_token");
    }
  }

  const latencySeries = useMemo(
    () =>
      samples.slice(-150).map((s, i) => ({
        idx: i + 1,
        latency: s.latencyMs,
      })),
    [samples]
  );

  const statusData = useMemo(() => {
    const source = latestSummary?.statusCounts ?? {};
    return Object.entries(source).map(([status, count]) => ({ status, count }));
  }, [latestSummary]);

  function parseKV(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const line of text.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const idx = trimmed.indexOf(":");
      if (idx < 0) continue;
      out[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim();
    }
    return out;
  }

  function requireToken() {
    if (!authToken.trim()) {
      throw new Error("Enter the auth token configured in backend AUTH_TOKEN.");
    }
    return authToken.trim();
  }

  function buildMultipartFields() {
    return multipartRows.reduce<Record<string, string>>((acc, row) => {
      if (row.enabled && row.kind === "text" && row.fieldName.trim()) {
        acc[row.fieldName.trim()] = row.value;
      }
      return acc;
    }, {});
  }

  function buildMultipartFiles() {
    return multipartRows
      .filter((row) => row.enabled && row.kind === "file" && row.fieldName.trim() && row.fileId)
      .map((row) => ({
        fieldName: row.fieldName.trim(),
        fileId: row.fileId,
        fileName: row.fileName,
      }));
  }

  function updateMultipartRow(id: string, patch: Partial<MultipartRow>) {
    setMultipartRows((rows) => rows.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function addMultipartRow(kind: "text" | "file") {
    setMultipartRows((rows) => [
      ...rows,
      {
        id: crypto.randomUUID(),
        enabled: true,
        kind,
        fieldName: kind === "file" ? "file" : "name",
        value: "",
        fileId: "",
        fileName: "",
      },
    ]);
  }

  function withAuth<T>(action: (token: string) => Promise<T>, after?: (result: T) => void) {
    void (async () => {
      try {
        setBackendError("");
        const result = await action(requireToken());
        after?.(result);
      } catch (error) {
        setBackendError(error instanceof Error ? error.message : "Authenticated request failed");
      }
    })();
  }

  async function handleStartRun() {
    try {
      setBackendError("");
      const token = requireToken();
      const nextConfig: RequestConfig = {
        ...config,
        headers: parseKV(headersText),
        body: {
          ...config.body,
          form: parseKV(formBodyText),
          multipartFields: buildMultipartFields(),
          multipartFiles: buildMultipartFiles(),
        },
        successStatuses: successStatusesText
          .split(",")
          .map((v) => Number(v.trim()))
          .filter((v) => Number.isInteger(v)),
        userAgent: {
          ...config.userAgent,
          customList: customUaText.split("\n").map((v) => v.trim()).filter(Boolean),
        },
      };
      const started = await startRun(nextConfig, token);
      setRunId(started.runId);
      setSamples([]);
      setLatestSummary(null);
      setRunning(true);
      setTab("live");
      const unsubscribe = streamRun(
        started.runId,
        token,
        (event) => {
          const data = event as { type: string; sample?: RunSample; summary?: RunSummary };
          if (data.type === "sample" && data.sample) {
            setSamples((prev) => [...prev, data.sample as RunSample]);
          }
          if (data.type === "summary" && data.summary) {
            setLatestSummary(data.summary as RunSummary);
            setRunning(false);
            void loadReports();
            unsubscribe();
          }
        },
        (error) => {
          setRunning(false);
          setBackendError(error.message);
        }
      );
    } catch (error) {
      setBackendError(error instanceof Error ? error.message : "Failed to start run");
    }
  }

  async function handleMultipartUpload(rowId: string, file: File | null) {
    if (!file) return;
    try {
      const res = await uploadFile(file, requireToken());
      updateMultipartRow(rowId, { fileId: res.fileId, fileName: res.fileName });
    } catch (error) {
      setBackendError(error instanceof Error ? error.message : "Upload failed");
    }
  }

  async function loadProxies() {
    try {
      const data = await listProxies(requireToken());
      setProxies(Array.isArray(data) ? data : []);
    } catch (error) {
      setBackendError(error instanceof Error ? error.message : "Could not load proxies");
    }
  }

  async function loadReports() {
    try {
      setReports(await listReports(requireToken()));
    } catch (error) {
      setBackendError(error instanceof Error ? error.message : "Could not load reports");
    }
  }

  return (
    <main className="min-h-screen bg-[#09090b] text-zinc-100">
      <div className="mx-auto flex max-w-7xl flex-col gap-6 p-6">
        <header className="rounded-2xl border border-zinc-800 bg-gradient-to-r from-zinc-900 to-zinc-950 p-6">
          <h1 className="text-2xl font-semibold tracking-tight">Simple Network Tester</h1>
          <p className="mt-2 text-sm text-zinc-400">
            Dark, local-first load tester with custom success statuses, proxy rotation, and sleek real-time reporting.
          </p>
          {backendError ? <p className="mt-2 text-sm text-red-400">{backendError}</p> : null}
          <div className="mt-4 grid gap-2 md:max-w-xl">
            <label className="text-xs font-medium uppercase tracking-wide text-zinc-500" htmlFor="auth-token">
              Auth token
            </label>
            <Input
              id="auth-token"
              type="password"
              autoComplete="current-password"
              placeholder="Token from backend AUTH_TOKEN"
              value={authToken}
              onChange={(e) => saveAuthToken(e.target.value)}
            />
            <p className="text-xs text-zinc-500">
              Stored in this browser session only and sent as a Bearer token. It is not bundled in frontend env.
            </p>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            {(["builder", "live", "proxies", "reports"] as const).map((item) => (
              <Button key={item} variant={tab === item ? "default" : "secondary"} onClick={() => setTab(item)}>
                {item.toUpperCase()}
              </Button>
            ))}
          </div>
        </header>

        {tab === "builder" ? (
          <section className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Request Builder</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <Field label="Test name" htmlFor="test-name">
                  <Input id="test-name" placeholder="Quick Test" value={config.name} onChange={(e) => setConfig((p) => ({ ...p, name: e.target.value }))} />
                </Field>
                <div className="grid grid-cols-3 gap-3">
                  <Field label="Method" htmlFor="method">
                    <Select id="method" value={config.method} onChange={(e) => setConfig((p) => ({ ...p, method: e.target.value as "GET" | "POST" }))}>
                      <option value="GET">GET</option>
                      <option value="POST">POST</option>
                    </Select>
                  </Field>
                  <Field className="col-span-2" label="Request URL" htmlFor="request-url">
                    <Input id="request-url" placeholder="https://your-app/api" value={config.url} onChange={(e) => setConfig((p) => ({ ...p, url: e.target.value }))} />
                  </Field>
                </div>
                <Field label="Headers" htmlFor="headers">
                  <Textarea id="headers" rows={4} placeholder={"Header-Name: value\nX-Trace: load-test"} value={headersText} onChange={(e) => setHeadersText(e.target.value)} />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Concurrency" htmlFor="concurrency">
                    <Input id="concurrency" type="number" min={1} value={config.concurrency} onChange={(e) => setConfig((p) => ({ ...p, concurrency: Number(e.target.value) }))} />
                  </Field>
                  <Field label="Timeout (ms)" htmlFor="timeout">
                    <Input id="timeout" type="number" min={1} value={config.timeoutMs} onChange={(e) => setConfig((p) => ({ ...p, timeoutMs: Number(e.target.value) }))} />
                  </Field>
                  <Field label="Request count" htmlFor="request-count">
                    <Input id="request-count" type="number" min={0} value={config.requests} onChange={(e) => setConfig((p) => ({ ...p, requests: Number(e.target.value) }))} />
                  </Field>
                  <Field label="Duration (seconds)" htmlFor="duration">
                    <Input id="duration" type="number" min={0} value={config.durationSeconds} onChange={(e) => setConfig((p) => ({ ...p, durationSeconds: Number(e.target.value) }))} />
                  </Field>
                </div>
                <Field label="Success statuses" htmlFor="success-statuses">
                  <Input id="success-statuses" placeholder="200,400" value={successStatusesText} onChange={(e) => setSuccessStatusesText(e.target.value)} />
                </Field>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Body, User-Agent, Proxy</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <Field label="Body type" htmlFor="body-type">
                  <Select id="body-type" value={config.body.type} onChange={(e) => setConfig((p) => ({ ...p, body: { ...p.body, type: e.target.value as RequestConfig["body"]["type"] } }))}>
                    <option value="none">No Body</option>
                    <option value="json">JSON</option>
                    <option value="form">Form URL Encoded</option>
                    <option value="multipart">Multipart + Files</option>
                  </Select>
                </Field>
                {config.body.type === "json" ? (
                  <Field label="JSON body" htmlFor="json-body">
                    <Textarea id="json-body" rows={6} value={config.body.json} onChange={(e) => setConfig((p) => ({ ...p, body: { ...p.body, json: e.target.value } }))} />
                  </Field>
                ) : null}
                {config.body.type === "form" ? (
                  <Field label="Form fields" htmlFor="form-body">
                    <Textarea id="form-body" rows={6} placeholder={"email: demo@example.com\nname: Jane"} value={formBodyText} onChange={(e) => setFormBodyText(e.target.value)} />
                  </Field>
                ) : null}
                {config.body.type === "multipart" ? (
                  <div className="space-y-3 rounded-lg border border-zinc-800 bg-zinc-950/60 p-3">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-zinc-200">Multipart fields</p>
                        <p className="text-xs text-zinc-500">Insomnia-style rows. Rename file fields and add as many file rows as needed.</p>
                      </div>
                      <div className="flex gap-2">
                        <Button type="button" variant="secondary" size="sm" onClick={() => addMultipartRow("text")}>
                          Add Text
                        </Button>
                        <Button type="button" variant="secondary" size="sm" onClick={() => addMultipartRow("file")}>
                          Add File
                        </Button>
                      </div>
                    </div>
                    <div className="overflow-x-auto">
                      <div className="min-w-[720px] space-y-2">
                        <div className="grid grid-cols-[70px_110px_180px_1fr_70px] gap-2 px-2 text-xs uppercase tracking-wide text-zinc-500">
                          <span>Use</span>
                          <span>Type</span>
                          <span>Field name</span>
                          <span>Value / File</span>
                          <span>Remove</span>
                        </div>
                        {multipartRows.map((row) => (
                          <div key={row.id} className="grid grid-cols-[70px_110px_180px_1fr_70px] items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900/80 p-2">
                            <label className="flex items-center gap-2 text-xs text-zinc-400">
                              <input
                                aria-label={`Enable multipart ${row.fieldName || row.id}`}
                                type="checkbox"
                                checked={row.enabled}
                                onChange={(e) => updateMultipartRow(row.id, { enabled: e.target.checked })}
                              />
                              On
                            </label>
                            <Field label="Row type" srOnly htmlFor={`multipart-kind-${row.id}`}>
                              <Select
                                id={`multipart-kind-${row.id}`}
                                value={row.kind}
                                onChange={(e) => updateMultipartRow(row.id, { kind: e.target.value as "text" | "file" })}
                              >
                                <option value="text">Text</option>
                                <option value="file">File</option>
                              </Select>
                            </Field>
                            <Field label="Multipart field name" srOnly htmlFor={`multipart-name-${row.id}`}>
                              <Input
                                id={`multipart-name-${row.id}`}
                                placeholder="field name"
                                value={row.fieldName}
                                onChange={(e) => updateMultipartRow(row.id, { fieldName: e.target.value })}
                              />
                            </Field>
                            {row.kind === "text" ? (
                              <Field label="Multipart field value" srOnly htmlFor={`multipart-value-${row.id}`}>
                                <Input
                                  id={`multipart-value-${row.id}`}
                                  placeholder="value"
                                  value={row.value}
                                  onChange={(e) => updateMultipartRow(row.id, { value: e.target.value })}
                                />
                              </Field>
                            ) : (
                              <Field label="Multipart file" srOnly htmlFor={`multipart-file-${row.id}`}>
                                <div className="grid grid-cols-[1fr_auto] items-center gap-2">
                                  <Input
                                    id={`multipart-file-${row.id}`}
                                    type="file"
                                    onChange={(e) => void handleMultipartUpload(row.id, e.target.files?.[0] ?? null)}
                                  />
                                  <Badge>{row.fileName || "No file"}</Badge>
                                </div>
                              </Field>
                            )}
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => setMultipartRows((rows) => rows.filter((item) => item.id !== row.id))}
                            >
                              Delete
                            </Button>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                ) : null}
                <Field label="User-Agent mode" htmlFor="ua-mode">
                  <Select id="ua-mode" value={config.userAgent.mode} onChange={(e) => setConfig((p) => ({ ...p, userAgent: { ...p.userAgent, mode: e.target.value as RequestConfig["userAgent"]["mode"] } }))}>
                    <option value="default_rotate">Rotate Default Fake User Agents</option>
                    <option value="custom_rotate">Rotate Custom List</option>
                    <option value="fixed">Fixed User Agent</option>
                  </Select>
                </Field>
                {config.userAgent.mode === "fixed" ? (
                  <Field label="Fixed User-Agent" htmlFor="fixed-ua">
                    <Input id="fixed-ua" value={config.userAgent.fixedValue} placeholder="Mozilla/5.0 ..." onChange={(e) => setConfig((p) => ({ ...p, userAgent: { ...p.userAgent, fixedValue: e.target.value } }))} />
                  </Field>
                ) : null}
                {config.userAgent.mode === "custom_rotate" ? (
                  <Field label="Custom User-Agent list" htmlFor="custom-ua">
                    <Textarea id="custom-ua" rows={4} placeholder={"One user-agent per line"} value={customUaText} onChange={(e) => setCustomUaText(e.target.value)} />
                  </Field>
                ) : null}
                <div className="flex items-center justify-between rounded-md border border-zinc-800 p-3">
                  <span className="text-sm">Enable proxy usage for this run</span>
                  <Switch checked={config.proxy.enabled} onCheckedChange={(v) => setConfig((p) => ({ ...p, proxy: { enabled: v } }))} />
                </div>
                <Button className="w-full" onClick={() => void handleStartRun()}>
                  Start Load Test
                </Button>
              </CardContent>
            </Card>
          </section>
        ) : null}

        {tab === "live" ? (
          <section className="grid gap-6 xl:grid-cols-3">
            <Card className="xl:col-span-2">
              <CardHeader>
                <CardTitle>Latency Timeline {runId ? <Badge className="ml-2">{runId}</Badge> : null}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="h-72">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={latencySeries}>
                      <XAxis dataKey="idx" stroke="#71717a" />
                      <YAxis stroke="#71717a" />
                      <Tooltip />
                      <Line type="monotone" dataKey="latency" stroke="#6366f1" dot={false} strokeWidth={2} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Live Summary</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <div className="flex justify-between"><span>Status</span><Badge>{running ? "RUNNING" : "IDLE"}</Badge></div>
                <div className="flex justify-between"><span>Samples</span><span>{samples.length}</span></div>
                <div className="flex justify-between"><span>Avg (live)</span><span>{samples.length ? Math.round(samples.reduce((a, s) => a + s.latencyMs, 0) / samples.length) : 0} ms</span></div>
                {latestSummary ? (
                  <>
                    <div className="mt-2 h-px bg-zinc-800" />
                    <div className="flex justify-between"><span>Total</span><span>{latestSummary.totalRequests}</span></div>
                    <div className="flex justify-between"><span>Errors</span><span>{latestSummary.errorCount}</span></div>
                    <div className="flex justify-between"><span>P95</span><span>{latestSummary.p95LatencyMs} ms</span></div>
                    <div className="flex justify-between"><span>RPS</span><span>{latestSummary.rps.toFixed(2)}</span></div>
                  </>
                ) : null}
              </CardContent>
            </Card>
            <Card className="xl:col-span-3">
              <CardHeader>
                <CardTitle>Status Distribution</CardTitle>
              </CardHeader>
              <CardContent className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie data={statusData} dataKey="count" nameKey="status" outerRadius={95}>
                      {statusData.map((entry, index) => (
                        <Cell key={entry.status} fill={chartColors[index % chartColors.length]} />
                      ))}
                    </Pie>
                    <Tooltip />
                  </PieChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          </section>
        ) : null}

        {tab === "proxies" ? (
          <section className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Proxy Manager</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <Field label="Proxy list" htmlFor="proxy-list">
                  <Textarea id="proxy-list" rows={8} placeholder={"Paste proxies, one per line\nip:port\nhttp://user:pass@ip:port"} value={proxyText} onChange={(e) => setProxyText(e.target.value)} />
                </Field>
                <div className="grid grid-cols-2 gap-2">
                  <Button onClick={() => withAuth((token) => importProxyList(proxyText, token), () => void loadProxies())}>Import List</Button>
                  <Button
                    variant="secondary"
                    onClick={() =>
                      withAuth((token) => recheckProxies(token), (data) => setProxies(Array.isArray(data) ? data : []))
                    }
                  >
                    Recheck Validity
                  </Button>
                  <Button variant="secondary" onClick={() => void loadProxies()}>
                    Refresh
                  </Button>
                  <Button variant="destructive" onClick={() => withAuth((token) => deleteInactiveProxies(token), () => void loadProxies())}>
                    Delete Inactive
                  </Button>
                </div>
                <div className="flex items-center justify-between rounded-md border border-zinc-800 p-3">
                  <span className="text-sm">Enable all proxies globally</span>
                  <Switch
                    checked={proxiesEnabled}
                    onCheckedChange={(v) =>
                      withAuth(
                        (token) => setProxiesEnabled(v, token),
                        () => {
                          setProxiesEnabledState(v);
                          void loadProxies();
                        }
                      )
                    }
                  />
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Loaded Proxies ({safeProxies.length})</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {safeProxies.slice(0, 40).map((p) => (
                  <div key={p.address} className="rounded-md border border-zinc-800 p-3 text-xs">
                    <div className="flex items-center justify-between">
                      <code className="text-zinc-300">{p.address}</code>
                      <Badge className={p.active ? "border-green-700 bg-green-950 text-green-300" : "border-red-700 bg-red-950 text-red-300"}>
                        {p.active ? "active" : "inactive"}
                      </Badge>
                    </div>
                    <p className="mt-1 text-zinc-500">latency: {p.lastLatency} ms | failures: {p.failureCount}</p>
                    {p.lastError ? <p className="mt-1 text-red-400">{p.lastError}</p> : null}
                  </div>
                ))}
              </CardContent>
            </Card>
          </section>
        ) : null}

        {tab === "reports" ? (
          <section className="space-y-4">
            <div className="flex justify-end">
              <Button variant="secondary" onClick={() => void loadReports()}>
                Refresh Reports
              </Button>
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              {reports.map((r) => (
                <Card key={r.runId}>
                  <CardHeader>
                    <CardTitle className="flex items-center justify-between">
                      <span>{r.config.name || r.runId}</span>
                      <button
                        type="button"
                        className="text-xs text-indigo-300 underline"
                        onClick={() => withAuth((token) => downloadReportCsv(r.runId, token))}
                      >
                        CSV
                      </button>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="grid grid-cols-2 gap-2 text-sm text-zinc-300">
                    <span>Total</span><span>{r.totalRequests}</span>
                    <span>Errors</span><span>{r.errorCount}</span>
                    <span>Avg Latency</span><span>{r.avgLatencyMs.toFixed(2)} ms</span>
                    <span>P95</span><span>{r.p95LatencyMs} ms</span>
                    <span>RPS</span><span>{r.rps.toFixed(2)}</span>
                    <span>Target</span><span className="truncate">{r.config.url}</span>
                  </CardContent>
                </Card>
              ))}
            </div>
          </section>
        ) : null}
      </div>
    </main>
  );
}

function Field({
  label,
  htmlFor,
  children,
  className,
  srOnly = false,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
  className?: string;
  srOnly?: boolean;
}) {
  return (
    <div className={className}>
      <label
        className={srOnly ? "sr-only" : "mb-1.5 block text-xs font-medium uppercase tracking-wide text-zinc-500"}
        htmlFor={htmlFor}
      >
        {label}
      </label>
      {children}
    </div>
  );
}
