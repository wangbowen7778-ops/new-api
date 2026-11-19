export const meta = {
  apiVersion: 1,
  key: "sora",
  name: "Sora",
  icon: "Sora.Color",
  description: {
    en: "OpenAI Sora video generation (text-to-video, image-to-video, and remix)",
    zh: "OpenAI Sora 视频生成（文生视频、图生视频、remix）",
  },
  version: "1.1.0",
  channelTypes: [55, 1], // OpenAI-type channels natively serve sora with the same wire format
  author: { name: "QuantumNous" },
  models: ["sora-2", "sora-2-pro"],
  fetchMode: "per_task",
  // A New API gateway serves /v1/videos as a host protocol, so no URL changes.
  upstreams: ["vendor", "new_api"],
  usageSchema: {
    // Requested video duration in seconds.
    seconds: {
      type: "number",
      unit: "second",
      description: { en: "Video generation unit price", zh: "视频生成单价" },
    },
    // Requested output video dimensions.
    size: {
      enum: ["720x1280", "1280x720", "1792x1024", "1024x1792"],
      enumLabels: {
        "720x1280": { en: "720x1280", zh: "720x1280" },
        "1280x720": { en: "1280x720", zh: "1280x720" },
        "1792x1024": { en: "1792x1024", zh: "1792x1024" },
        "1024x1792": { en: "1024x1792", zh: "1024x1792" },
      },
      description: { en: "Output video dimensions", zh: "输出视频尺寸" },
    },
    // Billing tier derived from `size`. Declared so tiered expressions can key
    // off `u("resolution")` instead of having to know pixel dimensions.
    resolution: {
      enum: ["480P", "720P", "1080P"],
      enumLabels: {
        "480P": { en: "480P", zh: "480P" },
        "720P": { en: "720P", zh: "720P" },
        "1080P": { en: "1080P", zh: "1080P" },
      },
      description: { en: "Billing tier derived from output size", zh: "由输出尺寸推导的计费档位" },
    },
  },
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }, "openai_video"],
};

function trimmed(value) {
  return String(value || "").trim();
}

// size → 计费档位。缺失映射时返回空串，由计费表达式自行决定如何兜底。
const SIZE_RESOLUTION = {
  "720x1280": "720P",
  "1280x720": "720P",
  "1792x1024": "1080P",
  "1024x1792": "1080P",
};

const RESOLUTION_TIERS = ["480P", "720P", "1080P"];

function resolutionForSize(size) {
  return SIZE_RESOLUTION[trimmed(size)] || "";
}

// 客户端显式传的 resolution 优先（TD 协议用它定档），大小写不敏感。
function explicitResolution(req) {
  const value = trimmed(req && req.resolution).toUpperCase();
  return RESOLUTION_TIERS.includes(value) ? value : "";
}

// 按名字大小写不敏感地取客户端请求头（Go 侧会规范化成 X-Td-Contract）。
function clientHeader(headers, wanted) {
  if (!headers) return "";
  const target = wanted.toLowerCase();
  for (const key of Object.keys(headers)) {
    if (key.toLowerCase() === target) return trimmed(headers[key]);
  }
  return "";
}

function responsesInput(req) {
  const texts = [],
    images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!item || typeof item !== "object" || Array.isArray(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!part || typeof part !== "object" || Array.isArray(part)) continue;
        if (["input_text", "text"].includes(part.type) && typeof part.text === "string") texts.push(part.text);
        if (["input_image", "image_url"].includes(part.type)) {
          let image = part.image_url;
          if (image && typeof image === "object") image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return {
    prompt: texts
      .filter(function (text) {
        return trimmed(text);
      })
      .join("\n"),
    images: images,
  };
}

function responsesVideoText(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

function requestValues(req, model) {
  const values = {};
  for (const key of Object.keys(req || {})) {
    // 下划线开头的字段是网关注入的计费元数据（如 _reference_seconds），
    // 只参与计费，不转发给上游。
    if (key.charAt(0) === "_") continue;
    values[key] = req[key];
  }
  values.model = model;
  return values;
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  if (!String(req.prompt || "").trim()) throw new Error("field prompt is required");
  const action = ctx.action === "remix" ? "remix" : ctx.action;
  const headers = { Authorization: "Bearer " + ctx.apiKey };
  // TD 契约头必须原样带到上游：上游据此走 TD 视频协议分支，
  // 缺了它会把 duration/resolution 当成非法参数直接 400。
  const contract = clientHeader(ctx.requestHeaders, "X-TD-Contract");
  if (contract) headers["X-TD-Contract"] = contract;
  if (action === "remix") {
    headers["Content-Type"] = "application/json";
    return { url: ctx.baseUrl + "/v1/videos/" + ctx.originTaskId + "/remix", method: "POST", headers, body: requestValues(req, ctx.upstreamModel), action };
  }
  if ((ctx.files || []).length) {
    const parts = [];
    const values = requestValues(req, ctx.upstreamModel);
    for (const key of Object.keys(values)) {
      const value = values[key];
      if (value === undefined || value === null) continue;
      if (typeof value === "object" && !Array.isArray(value)) {
        // multipart 只能放字符串，嵌套对象（metadata、parameters）序列化后带上，
        // 否则这类参数会在带参考图的请求里静默丢失。
        parts.push({ name: key, value: JSON.stringify(value) });
      } else if (typeof value !== "object") {
        parts.push({ name: key, value: value });
      }
    }
    for (const file of ctx.files) parts.push({ name: file.field, fileRef: file.ref, filename: file.filename });
    return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, bodyType: "multipart", parts };
  }
  headers["Content-Type"] = "application/json";
  return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, body: requestValues(req, ctx.upstreamModel) };
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const taskId = body.id || body.task_id;
  if (!taskId) throw new Error("task_id is empty");
  return { taskId, taskData: body };
}

export function extractUsage(ctx) {
  if (ctx.action === "remix") return {};
  const req = ctx.requestBody || {};
  let seconds = Number(req.seconds || req.duration || 4);
  if (!Number.isFinite(seconds) || (seconds <= 0 && seconds !== -1)) seconds = 4;
  // -1 为自动时长：上游按模型默认档生成，计费快照按各模型默认时长预估
  // （wan3.0-video-prime 默认 10 秒，wan3.0-video 默认 5 秒）。
  if (seconds === -1) seconds = String(req.model || "").indexOf("prime") >= 0 ? 10 : 5;
  // 预扣费口径：成片（输出）+ 参考视频（输入，本地精确探测）。
  // 拆分写入 usage_facts，供任务日志展示输入/输出的计费构成。
  const outputSeconds = Math.min(seconds, 3600);
  const reference = Number(req._reference_seconds || 0);
  const referenceSeconds = Number.isFinite(reference) && reference > 0 ? Math.round(reference * 100) / 100 : 0;
  const totalSeconds = Math.min(outputSeconds + referenceSeconds, 3600);
  const explicit = explicitResolution(req);
  // 没传 resolution 也没传 size 时，沿用 sora 协议的历史默认值 720x1280，
  // 避免计费表达式落到 else 的最贵档；一旦显式给了 resolution 就不再伪造 size。
  const size = trimmed(req.size) || (explicit ? "" : "720x1280");
  const facts = { seconds: totalSeconds, output_seconds: outputSeconds };
  if (referenceSeconds > 0) facts.input_video_seconds = referenceSeconds;
  if (size) facts.size = size;
  // 计费表达式按档位判断，必须把档位一起回传，否则 u("resolution")
  // 取到空值会落到表达式的 else 分支（最贵档）。
  const resolution = explicit || resolutionForSize(size);
  if (resolution) facts.resolution = resolution;
  return facts;
}

export function extractUsageOnComplete(task, taskResult, body) {
  const facts = {};
  const source = body || {};
  // 最终结算对齐上游口径：优先读响应里的 usage 块（input_seconds/output_seconds
  // 是上游回报的最终计费值）；缺失或非 final 语义时回退到 seconds/duration，
  // 保持预扣金额不变。
  const usage = source.usage && typeof source.usage === "object" ? source.usage : null;
  const usageUsable = usage !== null && (usage.usage_semantic === undefined || usage.usage_semantic === "final");
  const inputSeconds = usageUsable ? Number(usage.input_seconds || 0) : 0;
  const outputSeconds = usageUsable ? Number(usage.output_seconds || 0) : 0;
  if (Number.isFinite(inputSeconds) && Number.isFinite(outputSeconds) && inputSeconds + outputSeconds > 0) {
    facts.seconds = Math.min(inputSeconds + outputSeconds, 3600);
    if (inputSeconds > 0) facts.input_video_seconds = inputSeconds;
    if (outputSeconds > 0) facts.output_seconds = outputSeconds;
  } else {
    const seconds = Number(source.seconds || source.duration || 0);
    if (Number.isFinite(seconds) && seconds > 0) facts.seconds = Math.min(seconds, 3600);
  }
  const explicit = explicitResolution(source);
  const size = trimmed(source.size);
  if (["720x1280", "1280x720", "1792x1024", "1024x1792"].includes(size)) facts.size = size;
  const resolution = explicit || resolutionForSize(size);
  if (resolution) facts.resolution = resolution;
  return facts;
}

export function buildQueryRequest(ctx) {
  return { url: ctx.baseUrl + "/v1/videos/" + ctx.taskId, method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
}

export function parseTaskResult(ctx, body) {
  const statuses = {
    queued: "QUEUED",
    pending: "QUEUED",
    processing: "IN_PROGRESS",
    in_progress: "IN_PROGRESS",
    completed: "SUCCESS",
    failed: "FAILURE",
    cancelled: "FAILURE",
  };
  const mapped = statuses[body.status];
  const result = { status: mapped || "UNKNOWN" };
  if (!mapped) result.reason = "unrecognized status: " + String(body.status || "");
  if (body.progress > 0 && body.progress < 100) result.progress = body.progress + "%";
  if (result.status === "FAILURE") result.reason = body.error && body.error.message ? body.error.message : "task failed";
  return result;
}

export function listArtifacts(task) {
  return task.status === "SUCCESS" ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  return {
    url: ctx.baseUrl + "/v1/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/content",
    method: ctx.clientRequest.method,
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const req = ctx.body.value;
      if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      const model = trimmed(req.model);
      if (!model) throw new Error("model is required");
      if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
      if (req.images !== undefined && !Array.isArray(req.images)) throw new Error("images must be an array");
      if (req.metadata !== undefined && (!req.metadata || typeof req.metadata !== "object" || Array.isArray(req.metadata)))
        throw new Error("metadata must be an object");
      const input = responsesInput(req);
      const prompt = input.prompt || trimmed(req.prompt);
      if (!prompt) throw new Error("input is required");
      const images = [];
      for (const image of [req.image, req.input_reference].concat(req.images || [], input.images)) {
        if (trimmed(image) && !images.includes(trimmed(image))) images.push(trimmed(image));
      }
      const requestBody = { model: model, prompt: prompt };
      if (images.length) requestBody.input_reference = images[0];
      if (Object.prototype.hasOwnProperty.call(req, "seconds")) requestBody.seconds = req.seconds;
      else if (Object.prototype.hasOwnProperty.call(req, "duration")) requestBody.seconds = req.duration;
      if (Object.prototype.hasOwnProperty.call(req, "size")) requestBody.size = req.size;
      if (Object.prototype.hasOwnProperty.call(req, "metadata")) requestBody.metadata = req.metadata;
      return { kind: "submit", model: model, action: images.length ? "image_to_video" : "text_to_video", requestBody: requestBody };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const value = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesVideoText(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE")
        return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }], state: state, done: true };
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx, _task) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesVideoText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "sora" },
      };
    },
  },
};

// 动作只用于任务日志与对账展示，不影响计费。多参考素材（多图、参考视频或
// mode=multi_ref）记为 reference_to_video，与百炼插件保持同一口径。
function videoTaskAction(req, hasUploadedReference) {
  const input = req && typeof req.input === "object" && !Array.isArray(req.input) ? req.input : {};
  const media = Array.isArray(input.media) ? input.media : [];
  let hasVideo = false;
  let images = 0;
  for (const item of media) {
    const type = item && typeof item === "object" ? String(item.type || "") : "";
    if (type === "reference_video") hasVideo = true;
    else if (type === "reference_image") images += 1;
  }
  if (hasVideo || images > 1 || req.mode === "multi_ref") return "reference_to_video";
  if (images === 1 || hasUploadedReference || req.input_reference || req.image) return "image_to_video";
  return "text_to_video";
}

const legacyRenderers = {
  openai_video: function (task) {
    const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
    const output = {
      id: task.task_id,
      object: "video",
      model: (task.properties || {}).origin_model_name || "",
      status: statuses[task.status] || "unknown",
      progress: Number(String(task.progress || "0").replace("%", "")),
      created_at: Number(task.created_at || 0),
    };
    const completedAt = Number(task.finished_at || task.updated_at || 0);
    if (completedAt > 0) output.completed_at = completedAt;
    if (task.status === "FAILURE") {
      output.error = { code: "video_generation_failed", message: "The video generation task failed." };
    }
    return output;
  },
};

protocols.openai_video = {
  decodeRequest: function (ctx) {
    if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
    if (ctx.body.kind === "json") {
      if (!ctx.body.value || Array.isArray(ctx.body.value)) throw new Error("JSON object required");
      const req = ctx.body.value;
      const seconds = req.seconds === undefined ? req.duration : req.seconds;
      // -1 为自动时长（wan3.0-video 支持），与 1–3600 一并放行
      if (seconds !== undefined && (!Number.isFinite(Number(seconds)) || (Number(seconds) <= 0 && Number(seconds) !== -1) || Number(seconds) > 3600))
        throw new Error("seconds must be -1 (auto) or between 1 and 3600");
      return {
        kind: "submit",
        model: ctx.model,
        action: videoTaskAction(req, false),
        requestBody: Object.assign({}, req, { model: ctx.model }),
      };
    }
    const first = function (name) {
      const values = (ctx.body.fields || {})[name] || [];
      if (values.length > 1) throw new Error(name + " must be provided once");
      return values[0];
    };
    const req = {};
    const fields = ctx.body.fields || {};
    for (const name of Object.keys(fields)) {
      req[name] = first(name);
    }
    let hasInputReferenceFile = false;
    for (const file of ctx.body.files || []) {
      if (file.field !== "input_reference") throw new Error("unexpected file field: " + file.field);
      if (hasInputReferenceFile) throw new Error("input_reference must be provided once");
      hasInputReferenceFile = true;
    }
    if (req.metadata !== undefined) {
      let parsed;
      try {
        parsed = JSON.parse(req.metadata);
      } catch (e) {
        throw new Error("metadata must be a JSON object string");
      }
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("metadata must be a JSON object string");
      req.metadata = parsed;
    }
    if (req.seconds !== undefined) req.seconds = Number(req.seconds);
    else if (req.duration !== undefined) req.seconds = Number(req.duration);
    const seconds = req.seconds === undefined ? req.duration : req.seconds;
    // -1 为自动时长（wan3.0-video 支持），与 1–3600 一并放行
    if (seconds !== undefined && (!Number.isFinite(Number(seconds)) || (Number(seconds) <= 0 && Number(seconds) !== -1) || Number(seconds) > 3600))
      throw new Error("seconds must be -1 (auto) or between 1 and 3600");
    return {
      kind: "submit",
      model: ctx.model,
      action: videoTaskAction(req, hasInputReferenceFile),
      requestBody: Object.assign({}, req, { model: ctx.model }),
    };
  },
  render: function (ctx, task) {
    if (task.data && typeof task.data === "object" && !Array.isArray(task.data)) return task.data;
    return legacyRenderers.openai_video(task);
  },
};
