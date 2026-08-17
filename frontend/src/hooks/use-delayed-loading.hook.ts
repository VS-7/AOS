import { useEffect, useState, useRef } from "react";

/**
 * Delays the loading state presentation to avoid skeleton flashes
 * on fast local/network responses.
 *
 * @param isLoading - The raw loading state from the data source
 * @param delayMs - Minimum delay before showing loading (default: 300ms)
 * @returns {boolean} Whether the UI should show the loading/skeleton state
 */
export function useDelayedLoading(isLoading: boolean, delayMs = 300): boolean {
  const [showLoading, setShowLoading] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!isLoading) {
      // Data arrived — cancel any pending show and hide immediately
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
      setShowLoading(false);
      return;
    }

    // Loading started — start the delay timer
    timeoutRef.current = setTimeout(() => {
      setShowLoading(true);
    }, delayMs);

    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, [isLoading, delayMs]);

  return showLoading;
}
