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
import { Download, Expand, Pencil, RotateCcw, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { copyToClipboard } from "@/lib/copy-to-clipboard";

import type { DrawingImage, DrawingRecord } from "../types";

interface DrawingGalleryProps {
  records: DrawingRecord[];
  onReuse: (record: DrawingRecord) => void;
  onDelete: (id: string) => void;
}

async function downloadImage(src: string, filename: string): Promise<void> {
  if (src.startsWith("data:")) {
    const link = document.createElement("a");
    link.href = src;
    link.download = filename;
    link.click();
    return;
  }

  try {
    const blob = await (await fetch(src)).blob();
    const objectUrl = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = objectUrl;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(objectUrl);
  } catch {
    window.open(src, "_blank", "noopener");
  }
}

function imageGridClass(count: number): string {
  if (count <= 1) return "grid-cols-1";
  if (count === 2) return "grid-cols-2";
  return "grid-cols-2 lg:grid-cols-3";
}

export function DrawingGallery({
  records,
  onReuse,
  onDelete,
}: DrawingGalleryProps) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState<DrawingImage | null>(null);

  const handleCopyPrompt = async (prompt: string) => {
    const copied = await copyToClipboard(prompt);
    if (copied) {
      toast.success(t("Prompt copied"));
    }
  };

  if (records.length === 0) {
    return (
      <div className="text-muted-foreground flex flex-1 flex-col items-center justify-center gap-2 py-16 text-sm">
        <p>{t("No images yet")}</p>
        <p className="text-xs">
          {t("Enter a prompt below to start generating")}
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="flex flex-col gap-6 pb-6">
        {records.map((record) => (
          <div
            key={record.id}
            className="bg-card border-border/70 rounded-xl border shadow-sm"
          >
            <div className="flex flex-wrap items-center gap-2 px-4 pt-3 pb-2">
              <Badge variant={record.mode === "edit" ? "secondary" : "default"}>
                {record.mode === "edit" ? t("Edit") : t("Generate")}
              </Badge>
              <span className="text-foreground/80 font-mono text-xs">
                {record.model}
              </span>
              {record.size && <Badge variant="outline">{record.size}</Badge>}
              {record.n > 1 && <Badge variant="outline">×{record.n}</Badge>}
              <div className="ml-auto flex items-center gap-1">
                <Button
                  aria-label={t("Reuse parameters")}
                  className="text-muted-foreground hover:text-foreground h-8 w-8"
                  onClick={() => onReuse(record)}
                  size="icon"
                  variant="ghost"
                >
                  <RotateCcw className="size-4" />
                </Button>
                <Button
                  aria-label={t("Delete record")}
                  className="text-muted-foreground hover:text-destructive h-8 w-8"
                  onClick={() => onDelete(record.id)}
                  size="icon"
                  variant="ghost"
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </div>

            <button
              className="text-muted-foreground hover:text-foreground line-clamp-2 px-4 pb-2 text-left text-sm"
              onClick={() => void handleCopyPrompt(record.prompt)}
              title={t("Click to copy prompt")}
              type="button"
            >
              {record.prompt}
            </button>

            <div
              className={`grid gap-2 px-4 pb-4 ${imageGridClass(record.images.length)}`}
            >
              {record.images.map((image) => (
                <div
                  key={image.id}
                  className="group border-border/60 relative flex items-center justify-center overflow-hidden rounded-lg border"
                >
                  <img
                    alt={image.revisedPrompt || record.prompt}
                    className="max-h-[70vh] max-w-full cursor-zoom-in object-contain"
                    onClick={() => setPreview(image)}
                    src={image.src}
                  />
                  <div className="absolute inset-0 flex items-end justify-end p-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <div className="flex gap-1">
                      <Button
                        aria-label={t("Preview")}
                        className="h-8 w-8"
                        onClick={() => setPreview(image)}
                        size="icon"
                        variant="secondary"
                      >
                        <Expand className="size-4" />
                      </Button>
                      <Button
                        aria-label={t("Download")}
                        className="h-8 w-8"
                        onClick={() =>
                          void downloadImage(
                            image.src,
                            `${record.model}-${image.id}.png`,
                          )
                        }
                        size="icon"
                        variant="secondary"
                      >
                        <Download className="size-4" />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      <Dialog
        onOpenChange={(open) => {
          if (!open) setPreview(null);
        }}
        open={preview !== null}
      >
        <DialogContent className="max-w-4xl sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-sm font-normal">
              <Pencil className="size-4" />
              {preview?.revisedPrompt || t("Preview")}
            </DialogTitle>
          </DialogHeader>
          {preview && (
            <div className="flex flex-col items-center gap-3">
              <img
                alt={preview.revisedPrompt || "image"}
                className="max-h-[70vh] max-w-full rounded-lg object-contain"
                src={preview.src}
              />
              <Button
                onClick={() =>
                  void downloadImage(preview.src, `drawing-${preview.id}.png`)
                }
              >
                <Download className="size-4" />
                {t("Download")}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
