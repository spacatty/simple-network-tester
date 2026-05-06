export type BodyType = "none" | "json" | "form" | "multipart";
export type UserAgentMode = "default_rotate" | "custom_rotate" | "fixed";

export type RequestConfig = {
  name: string;
  method: "GET" | "POST";
  url: string;
  headers: Record<string, string>;
  body: {
    type: BodyType;
    json: string;
    form: Record<string, string>;
    multipartFields: Record<string, string>;
    multipartFiles: Array<{ fieldName: string; fileId: string; fileName: string }>;
  };
  successStatuses: number[];
  timeoutMs: number;
  concurrency: number;
  requests: number;
  durationSeconds: number;
  rateLimitRps: number;
  userAgent: {
    mode: UserAgentMode;
    fixedValue: string;
    customList: string[];
  };
  proxy: { enabled: boolean };
};

export type RunSample = {
  runId: string;
  timestamp: string;
  latencyMs: number;
  statusCode: number;
  success: boolean;
  errorCategory: string;
  errorMessage: string;
  proxyAddress: string;
  bytesSent: number;
  bytesReceived: number;
};

export type RunSummary = {
  runId: string;
  state: string;
  startedAt: string;
  completedAt?: string;
  totalRequests: number;
  successCount: number;
  errorCount: number;
  avgLatencyMs: number;
  minLatencyMs: number;
  maxLatencyMs: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
  rps: number;
  statusCounts: Record<string, number>;
  errorBreakdown: Record<string, number>;
  config: RequestConfig;
};

export type ProxyRecord = {
  address: string;
  active: boolean;
  lastChecked?: string;
  lastLatency: number;
  lastError: string;
  successCount: number;
  failureCount: number;
};
