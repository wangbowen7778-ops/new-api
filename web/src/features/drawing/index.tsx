/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  ImageIcon,
  ImagePlus,
  Loader2,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { formatCurrencyFromUSD } from "@/lib/currency";
import { formatQuota } from "@/lib/format";

import {
  editImages,
  extractErrorMessage,
  generateImages,
  getPricing,
  getUserAccountInfo,
  getUserGroups,
} from "./api";
import { DrawingGallery } from "./components/drawing-gallery";
import {
  ASPECT_RATIOS,
  computeSize,
  DEFAULT_CONFIG,
  DRAWING_MODEL,
  ERROR_MESSAGES,
  FIXED_QUALITY,
  N_OPTIONS,
} from "./constants";
import {
  clearHistoryStorage,
  loadConfig,
  loadHistory,
  saveConfig,
  saveHistory,
} from "./lib/storage";
import type {
  DrawingConfig,
  DrawingGroupInfo,
  DrawingImage,
  DrawingMode,
  DrawingRecord,
  ImageApiData,
  PricingModelBrief,
} from "./types";

function createId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function toImageSrc(item: ImageApiData, outputFormat?: string): string | null {
  if (item.b64_json) {
    const mime = outputFormat === "jpeg" ? "image/jpeg" : "image/png";
    return `data:${mime};base64,${item.b64_json}`;
  }
  if (item.url) {
    return item.url;
  }
  return null;
}

/**
 * Turn a stored image (data URL or remote URL) into a File so it can be sent
 * as the reference image of an edit-style request.
 */
async function srcToFile(src: string): Promise<File | null> {
  try {
    const blob = await (await fetch(src)).blob();
    const type = blob.type || "image/png";
    const ext = type.split("/")[1] || "png";
    return new File([blob], `reference.${ext}`, { type });
  } catch {
    return null;
  }
}

function formatUsd(amount: number): string {
  return formatCurrencyFromUSD(amount, {
    digitsLarge: 4,
    digitsSmall: 4,
    abbreviate: false,
  });
}

export function Drawing() {
  const { t } = useTranslation();

  const [config, setConfig] = useState<DrawingConfig>(() => ({
    ...DEFAULT_CONFIG,
    ...loadConfig(),
  }));
  const [records, setRecords] = useState<DrawingRecord[]>([]);
  const [historyLoaded, setHistoryLoaded] = useState(false);
  const [account, setAccount] = useState<{
    quota: number;
    group: string;
  } | null>(null);
  const [pricingModels, setPricingModels] = useState<PricingModelBrief[]>([]);
  const [pricingGroupRatio, setPricingGroupRatio] = useState<
    Record<string, number>
  >({});
  const [groups, setGroups] = useState<Record<string, DrawingGroupInfo>>({});
  const [mode, setMode] = useState<DrawingMode>("generate");
  const [prompt, setPrompt] = useState("");
  const [editFile, setEditFile] = useState<File | null>(null);
  const [isGenerating, setIsGenerating] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const currentModel = DRAWING_MODEL;
  const currentSize = computeSize(config.ratio);
  // Most recent result, used as the conversational reference for the next
  // request when context reuse is on.
  const lastContextImage = records[0]?.images[0];
  const [currentSizeWidth, currentSizeHeight] = currentSize
    .split("x")
    .map((value) => Number.parseInt(value, 10));
  const groupOptions = Object.entries(groups).map(([name, info]) => ({
    value: name,
    label:
      typeof info.ratio === "number" && Number.isFinite(info.ratio)
        ? `${name} ×${info.ratio}`
        : name,
  }));

  const editFilePreviewUrl = useMemo(
    () => (editFile ? URL.createObjectURL(editFile) : null),
    [editFile],
  );

  useEffect(() => {
    return () => {
      if (editFilePreviewUrl) {
        URL.revokeObjectURL(editFilePreviewUrl);
      }
    };
  }, [editFilePreviewUrl]);

  useEffect(() => {
    saveConfig(config);
  }, [config]);

  useEffect(() => {
    let cancelled = false;
    void loadHistory().then((loaded) => {
      if (cancelled) return;
      setRecords((current) => {
        // Keep anything generated while the async load was in flight.
        const currentIds = new Set(current.map((record) => record.id));
        const merged = [
          ...current,
          ...loaded.filter((record) => !currentIds.has(record.id)),
        ];
        return merged.sort((a, b) => b.createdAt - a.createdAt);
      });
      setHistoryLoaded(true);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    // Skip the empty initial state so persistence never wipes the store
    // before the async load completes.
    if (historyLoaded) {
      saveHistory(records);
    }
  }, [records, historyLoaded]);

  const refreshAccount = useCallback(() => {
    void getUserAccountInfo().then(setAccount);
  }, []);

  useEffect(() => {
    refreshAccount();
    void getPricing().then((pricing) => {
      if (pricing.success && Array.isArray(pricing.data)) {
        setPricingModels(pricing.data);
      }
      if (pricing.group_ratio) {
        setPricingGroupRatio(pricing.group_ratio);
      }
    });
    void getUserGroups().then((res) => {
      if (res.success && res.data) {
        setGroups(res.data);
      }
    });
  }, [refreshAccount]);

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const updateConfig = useCallback(
    <K extends keyof DrawingConfig>(key: K, value: DrawingConfig[K]) => {
      setConfig((current) => ({ ...current, [key]: value }));
    },
    [],
  );

  // Resolve the default group once account and usable groups are loaded:
  // the saved choice wins if still usable, otherwise fall back to the
  // user's own group (or default).
  useEffect(() => {
    if (!account || Object.keys(groups).length === 0) return;
    if (config.group && config.group in groups) return;
    updateConfig("group", account.group in groups ? account.group : "default");
  }, [account, groups, config.group, updateConfig]);

  // Estimate: per-call unit price x group ratio x image count. The group is
  // the selected group (falling back to the user's own group). Only
  // per-request (quota_type 1) models have a per-call unit price; token-based
  // image models are billed by usage and cannot be estimated up front.
  const costEstimate = useMemo(() => {
    const pricing = pricingModels.find(
      (item) => item.model_name === currentModel,
    );
    const group = config.group || account?.group || "";
    const candidates = [
      groups[group]?.ratio,
      pricing?.group_ratio?.[group],
      pricingGroupRatio[group],
    ];
    let groupRatio = 1;
    for (const candidate of candidates) {
      if (typeof candidate === "number" && Number.isFinite(candidate)) {
        groupRatio = candidate;
        break;
      }
    }

    if (!pricing) {
      return { kind: "no-pricing" as const, groupRatio };
    }
    if (pricing.quota_type !== 1 || typeof pricing.model_price !== "number") {
      return { kind: "token-based" as const, groupRatio };
    }
    const unitPrice = pricing.model_price;
    const total = unitPrice * groupRatio * config.n;
    return {
      kind: "per-call" as const,
      groupRatio,
      unitPrice,
      total,
    };
  }, [
    pricingModels,
    pricingGroupRatio,
    groups,
    account,
    currentModel,
    config.group,
    config.n,
  ]);

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    setEditFile(file);
    if (file) {
      setMode("edit");
    }
  };

  const clearEditFile = () => {
    setEditFile(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleGenerate = async () => {
    if (!prompt.trim()) {
      toast.error(t(ERROR_MESSAGES.PROMPT_REQUIRED));
      return;
    }
    if (mode === "edit" && !editFile) {
      toast.error(t(ERROR_MESSAGES.IMAGE_REQUIRED));
      return;
    }

    const abortController = new AbortController();
    abortRef.current?.abort();
    abortRef.current = abortController;
    setIsGenerating(true);

    const finish = () => {
      if (abortRef.current === abortController) {
        abortRef.current = null;
      }
      setIsGenerating(false);
    };

    try {
      // Reference image: an explicit edit-mode upload wins; otherwise the
      // most recent generated image is reused as conversational context so
      // the next request can refine the previous result from its prompt.
      let referenceFile: File | null = mode === "edit" ? editFile : null;
      if (
        !referenceFile &&
        mode === "generate" &&
        (config.reuseContext ?? true) &&
        records[0]?.images[0]
      ) {
        referenceFile = await srcToFile(records[0].images[0].src);
        if (abortController.signal.aborted) return;
      }

      // Fan out one request per image instead of sending n in a single
      // request: the gpt-image-2 upstream rejects n > 1 with
      // "Unknown parameter: 'tools[0].n'". Billing is per image either way.
      const singleRequest = () => {
        if (referenceFile) {
          const form = new FormData();
          form.append("model", currentModel);
          form.append("prompt", prompt.trim());
          form.append("n", "1");
          form.append("size", currentSize);
          form.append("quality", FIXED_QUALITY);
          form.append("image", referenceFile);
          if (config.group) {
            form.append("group", config.group);
          }
          return editImages(form, abortController.signal);
        }
        return generateImages(
          {
            model: currentModel,
            prompt: prompt.trim(),
            n: 1,
            size: currentSize,
            quality: FIXED_QUALITY,
            group: config.group || undefined,
          },
          abortController.signal,
        );
      };

      const results = await Promise.allSettled(
        Array.from({ length: config.n }, () => singleRequest()),
      );

      if (abortController.signal.aborted) return;

      const images: DrawingImage[] = [];
      let firstError = "";
      let failedCount = 0;
      for (const result of results) {
        if (result.status === "rejected") {
          failedCount++;
          if (!firstError) {
            firstError = extractErrorMessage(result.reason);
          }
          continue;
        }
        const response = result.value;
        for (const item of response.data ?? []) {
          const src = toImageSrc(item, response.output_format);
          if (!src) continue;
          images.push({
            id: createId(),
            src,
            revisedPrompt: item.revised_prompt,
          });
        }
        if ((response.data ?? []).length === 0 && !firstError) {
          firstError =
            response.error?.message ||
            response.message ||
            t(ERROR_MESSAGES.API_REQUEST_ERROR);
        }
      }

      if (images.length === 0) {
        toast.error(firstError || t(ERROR_MESSAGES.API_REQUEST_ERROR));
        return;
      }
      if (failedCount > 0 || images.length < config.n) {
        toast.warning(
          `${images.length}/${config.n} ${t("Number of images")}`,
        );
      }

      const record: DrawingRecord = {
        id: createId(),
        mode: referenceFile ? "edit" : "generate",
        createdAt: Date.now(),
        prompt: prompt.trim(),
        model: currentModel,
        ratio: config.ratio,
        size: currentSize,
        n: config.n,
        group: config.group,
        images,
      };
      setRecords((current) => [record, ...current]);
      refreshAccount();
      setPrompt("");
      clearEditFile();
    } catch (error: unknown) {
      if (abortController.signal.aborted) return;
      toast.error(extractErrorMessage(error));
    } finally {
      finish();
    }
  };

  const handleStop = () => {
    abortRef.current?.abort();
    abortRef.current = null;
    setIsGenerating(false);
  };

  const handleReuse = (record: DrawingRecord) => {
    setMode(record.mode);
    setPrompt(record.prompt);
    if (record.ratio) {
      updateConfig("ratio", record.ratio);
    }
    if (record.group && record.group in groups) {
      updateConfig("group", record.group);
    }
    updateConfig("n", record.n);
  };

  const handleDeleteRecord = (id: string) => {
    setRecords((current) => current.filter((record) => record.id !== id));
  };

  const handleClearHistory = () => {
    setRecords([]);
    clearHistoryStorage();
  };

  return (
    <div className="flex size-full min-h-0 flex-col overflow-hidden md:flex-row">
      {/* Settings panel */}
      <aside className="border-border/70 bg-muted/20 w-full shrink-0 border-b p-4 md:w-72 md:overflow-y-auto md:border-r md:border-b-0">
        <div className="flex flex-col gap-4">
          <div className="bg-background grid grid-cols-2 gap-2 rounded-lg p-1 shadow-inner">
            <Button
              className="h-8"
              onClick={() => setMode("generate")}
              size="sm"
              variant={mode === "generate" ? "default" : "ghost"}
            >
              <Sparkles className="size-4" />
              {t("Generate")}
            </Button>
            <Button
              className="h-8"
              onClick={() => setMode("edit")}
              size="sm"
              variant={mode === "edit" ? "default" : "ghost"}
            >
              <ImagePlus className="size-4" />
              {t("Edit")}
            </Button>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-muted-foreground text-xs">
              {t("Group")}
            </Label>
            <Select
              items={groupOptions}
              onValueChange={(value) => {
                if (value !== null) {
                  updateConfig("group", value);
                }
              }}
              value={config.group}
            >
              <SelectTrigger className="w-full">
                <SelectValue>{config.group}</SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {groupOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-muted-foreground text-xs">
              {t("Model")}
            </Label>
            <div className="bg-background text-foreground/80 rounded-lg px-3 py-2 font-mono text-sm shadow-inner">
              {currentModel}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-muted-foreground text-xs">
              {t("Aspect ratio")}
            </Label>
            <div className="bg-background grid grid-cols-3 gap-2 rounded-lg p-1 shadow-inner">
              {ASPECT_RATIOS.map((ratio) => (
                <Button
                  className="h-8"
                  key={ratio}
                  onClick={() => updateConfig("ratio", ratio)}
                  size="sm"
                  variant={config.ratio === ratio ? "default" : "ghost"}
                >
                  {ratio}
                </Button>
              ))}
            </div>
            <p className="text-muted-foreground text-[11px]">
              {currentModel} · {currentSize}
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="text-muted-foreground text-xs">
              {t("Number of images")}
            </Label>
            <Select
              items={N_OPTIONS.map((n) => ({
                value: String(n),
                label: String(n),
              }))}
              onValueChange={(value) => updateConfig("n", Number(value) || 1)}
              value={String(config.n)}
            >
              <SelectTrigger className="w-full">
                <SelectValue>{config.n}</SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {N_OPTIONS.map((n) => (
                    <SelectItem key={n} value={String(n)}>
                      {n}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className="border-border/70 bg-background/60 flex flex-col gap-1 rounded-lg border p-3">
            <div className="text-muted-foreground text-xs">
              {t("Estimated cost")}
            </div>
            {costEstimate.kind === "per-call" && (
              <>
                <div className="text-foreground text-xs font-medium">
                  {formatUsd(costEstimate.unitPrice)} ×{" "}
                  {costEstimate.groupRatio.toFixed(2)} × {config.n} ={" "}
                  {formatUsd(costEstimate.total)}
                </div>
                <div className="text-muted-foreground text-[11px]">
                  {t("Unit price × group ratio × count")}
                </div>
              </>
            )}
            {costEstimate.kind === "token-based" && (
              <div className="text-muted-foreground text-[11px]">
                {t("Token-based billing, no per-call estimate")}
              </div>
            )}
            {costEstimate.kind === "no-pricing" && (
              <div className="text-muted-foreground text-[11px]">
                {t("No pricing configured for this model")}
              </div>
            )}
          </div>

          {account && (
            <div className="text-muted-foreground flex items-center justify-between text-xs">
              <span>{t("Current balance")}</span>
              <span className="text-foreground font-medium">
                {formatQuota(account.quota)}
              </span>
            </div>
          )}
        </div>
      </aside>

      {/* Main area: gallery + composer */}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <div className="border-border/70 flex items-center justify-between border-b px-4 py-2">
          <div className="text-foreground/80 flex items-center gap-2 text-sm font-medium">
            <ImageIcon className="size-4" />
            {t("Drawing")}
          </div>
          {records.length > 0 && (
            <Button
              className="text-muted-foreground hover:text-destructive h-8"
              onClick={handleClearHistory}
              size="sm"
              variant="ghost"
            >
              <Trash2 className="size-4" />
              {t("Clear history")}
            </Button>
          )}
        </div>

        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-4 pt-4">
          {isGenerating && (
            <div className="bg-card border-border/70 mb-6 overflow-hidden rounded-xl border shadow-sm">
              <div className="border-border/70 flex items-center gap-2 border-b px-4 py-2.5">
                <Sparkles className="text-muted-foreground size-4 animate-pulse" />
                <span className="text-foreground/80 text-sm font-medium">
                  {t("Generating...")}
                </span>
                <span className="text-muted-foreground ml-auto font-mono text-xs">
                  {currentSize} × {config.n}
                </span>
              </div>
              <div className="flex flex-wrap items-center justify-center gap-3 p-4">
                {Array.from({ length: config.n }, (_, index) => (
                  <div
                    className="skeleton-shimmer border-border/60 relative flex items-center justify-center overflow-hidden rounded-lg border"
                    key={index}
                    style={{
                      aspectRatio: `${currentSizeWidth} / ${currentSizeHeight}`,
                      height: config.n === 1 ? "40vh" : "20vh",
                      maxWidth: "100%",
                    }}
                  >
                    <Loader2 className="text-muted-foreground/70 size-6 animate-spin" />
                  </div>
                ))}
              </div>
            </div>
          )}
          <DrawingGallery
            onDelete={handleDeleteRecord}
            onReuse={handleReuse}
            records={records}
          />
        </div>

        {/* Composer */}
        <div className="border-border/70 bg-background/95 border-t p-3">
          <div className="mx-auto flex w-full max-w-3xl flex-col gap-2">
            {mode === "edit" ? (
              <div className="flex items-center gap-2">
                <input
                  accept="image/*"
                  className="hidden"
                  onChange={handleFileChange}
                  ref={fileInputRef}
                  type="file"
                />
                <Button
                  disabled={isGenerating}
                  onClick={() => fileInputRef.current?.click()}
                  size="sm"
                  variant="outline"
                >
                  <ImagePlus className="size-4" />
                  {t("Upload image")}
                </Button>
                {editFile && editFilePreviewUrl && (
                  <div className="border-border/70 bg-muted/30 flex items-center gap-2 rounded-lg border px-2 py-1">
                    <img
                      alt={editFile.name}
                      className="size-8 rounded object-cover"
                      src={editFilePreviewUrl}
                    />
                    <span className="max-w-40 truncate text-xs">
                      {editFile.name}
                    </span>
                    <Button
                      className="text-muted-foreground hover:text-destructive size-6"
                      onClick={clearEditFile}
                      size="icon"
                      variant="ghost"
                    >
                      <X className="size-3.5" />
                    </Button>
                  </div>
                )}
              </div>
            ) : (
              lastContextImage &&
              ((config.reuseContext ?? true) ? (
                <div className="border-border/70 bg-muted/30 flex items-center gap-2 self-start rounded-lg border px-2 py-1">
                  <img
                    alt={t("Last generated image")}
                    className="size-8 rounded object-cover"
                    src={lastContextImage.src}
                  />
                  <span className="text-muted-foreground text-xs">
                    {t("Refining the last result")}
                  </span>
                  <Button
                    aria-label={t("Stop reusing context")}
                    className="text-muted-foreground hover:text-destructive size-6"
                    disabled={isGenerating}
                    onClick={() => updateConfig("reuseContext", false)}
                    size="icon"
                    variant="ghost"
                  >
                    <X className="size-3.5" />
                  </Button>
                </div>
              ) : (
                <Button
                  className="self-start"
                  disabled={isGenerating || !lastContextImage}
                  onClick={() => updateConfig("reuseContext", true)}
                  size="sm"
                  variant="outline"
                >
                  <ImagePlus className="size-4" />
                  {t("Use last result as reference")}
                </Button>
              ))
            )}
            <div className="flex items-end gap-2">
              <Textarea
                disabled={isGenerating}
                onChange={(event) => setPrompt(event.target.value)}
                onKeyDown={(event) => {
                  if (
                    event.key === "Enter" &&
                    (event.ctrlKey || event.metaKey) &&
                    !isGenerating
                  ) {
                    void handleGenerate();
                  }
                }}
                placeholder={t("Describe the image you want...")}
                rows={2}
                value={prompt}
              />
              {isGenerating ? (
                <Button
                  className="border-destructive/25 bg-destructive/10 text-destructive hover:bg-destructive/15"
                  onClick={handleStop}
                  variant="secondary"
                >
                  {t("Stop")}
                </Button>
              ) : (
                <Button
                  disabled={!prompt.trim()}
                  onClick={() => void handleGenerate()}
                >
                  <Sparkles className="size-4" />
                  {t("Generate")}
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
