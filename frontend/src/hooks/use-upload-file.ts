import * as React from "react";
import { toast } from "sonner";

/**
 * A file handed back to the editor after "upload".
 *
 * The original posts to `uploadthing`, a hosted third-party service — ruled
 * out in docs/06 - Frontend/Design System.md as incompatible with an
 * offline-first, self-hosted product. Until `File (Go)` exists to persist an
 * upload for real, this reads the file locally and hands back a blob URL:
 * the image/video/file shows up in the document immediately, honestly, and
 * without a network call — the real limitation is that a blob URL does not
 * survive a page reload. The original had exactly this fallback for its own
 * unauthenticated path (`URL.createObjectURL`); this is that path, not a
 * mock of it.
 */
export interface UploadedFile {
  key: string;
  url: string;
  name: string;
  size: number;
  type: string;
}

interface UseUploadFileProps {
  onUploadComplete?: (file: UploadedFile) => void;
  onUploadError?: (error: unknown) => void;
}

export function useUploadFile({
  onUploadComplete,
  onUploadError,
}: UseUploadFileProps = {}) {
  const [uploadedFile, setUploadedFile] = React.useState<UploadedFile>();
  const [uploadingFile, setUploadingFile] = React.useState<File>();
  const [progress, setProgress] = React.useState(0);
  const [isUploading, setIsUploading] = React.useState(false);

  async function uploadFile(file: File): Promise<UploadedFile> {
    setIsUploading(true);
    setUploadingFile(file);
    setProgress(0);

    try {
      const local: UploadedFile = {
        key: `local-${Date.now()}-${file.name}`,
        url: URL.createObjectURL(file),
        name: file.name,
        size: file.size,
        type: file.type,
      };

      // There is nothing to await locally; the progress steps exist so the
      // upload-toast UI (which expects to animate) has something to show.
      for (let step = 0; step < 5; step++) {
        await new Promise((resolve) => setTimeout(resolve, 20));
        setProgress(Math.round(((step + 1) / 5) * 100));
      }

      setUploadedFile(local);
      onUploadComplete?.(local);
      return local;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The file could not be read.");
      onUploadError?.(error);
      throw error;
    } finally {
      setProgress(0);
      setIsUploading(false);
      setUploadingFile(undefined);
    }
  }

  return { isUploading, progress, uploadedFile, uploadFile, uploadingFile };
}
