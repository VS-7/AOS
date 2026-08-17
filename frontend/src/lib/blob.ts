/**
 * Converts a blob-backed payload into a Base64 data URL.
 *
 * @param blob - The source blob or file to serialize.
 * @returns A Base64 data URL, or `null` when the read fails.
 */
export async function blobToDataUrl(blob: Blob): Promise<string | null> {
  try {
    return await new Promise((resolve) => {
      const reader = new FileReader();

      reader.onloadend = () => resolve(reader.result as string);
      reader.onerror = () => resolve(null);
      reader.readAsDataURL(blob);
    });
  } catch {
    return null;
  }
}

/**
 * Fetches a blob URL and converts the payload into a Base64 data URL.
 *
 * @param url - The blob URL to read.
 * @returns A Base64 data URL, or `null` when the fetch or read fails.
 */
export async function blobUrlToDataUrl(url: string): Promise<string | null> {
  try {
    const response = await fetch(url);
    const blob = await response.blob();

    return blobToDataUrl(blob);
  } catch {
    return null;
  }
}
