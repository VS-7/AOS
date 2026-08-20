/**
 * `hooks/use-upload-file.ts` imports `OurFileRouter` from here — the
 * the original's uploadthing file-router definition, generated
 * server-side and re-exported to the frontend for type inference
 * (`UploadFilesOptions<OurFileRouter["editorUploader"]>`). AOS has no
 * uploadthing integration (it's a Wails desktop app with a local Go
 * backend, not a cloud upload SaaS), so there is no real router to mirror.
 *
 * Kept loose on purpose — see `vendor-stubs.d.ts`'s doc comment for why
 * `uploadthing` itself is a compile-only stub in this environment.
 */
export type OurFileRouter = {
  editorUploader: any;
  [key: string]: any;
};
