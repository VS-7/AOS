import { describe, it, expect } from "vitest";
import { microphoneErrorMessage, recordingMimeType } from "./use-chat-composer-audio";

const named = (name: string, message = "raw browser text") => Object.assign(new Error(message), { name });

describe("microphoneErrorMessage", () => {
  // The raw DOMException went straight to a toast: "Requested device not
  // found", in English, whatever the interface language.
  it("says what happened in words the interface owns", () => {
    expect(microphoneErrorMessage(named("NotFoundError"))).toMatch(/no microphone/i);
    expect(microphoneErrorMessage(named("NotAllowedError"))).toMatch(/denied/i);
    expect(microphoneErrorMessage(named("SecurityError"))).toMatch(/denied/i);
    expect(microphoneErrorMessage(named("NotReadableError"))).toMatch(/in use/i);
    expect(microphoneErrorMessage(named("NotSupportedError"))).toMatch(/not supported/i);
    expect(microphoneErrorMessage("anything")).not.toMatch(/raw browser text/);
  });
});

describe("recordingMimeType", () => {
  // WebKit's MediaRecorder does not record WebM, and asking for it throws
  // NotSupportedError in the desktop window.
  it("picks a format the engine can record, or leaves the choice to it", () => {
    expect(recordingMimeType((type) => type === "audio/mp4")).toBe("audio/mp4");
    expect(recordingMimeType((type) => type.startsWith("audio/webm"))).toBe("audio/webm;codecs=opus");
    expect(recordingMimeType(() => false)).toBeUndefined();
  });
});
