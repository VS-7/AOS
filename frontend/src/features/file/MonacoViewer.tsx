import "@/lib/monaco-setup";
import { useEffect, useState } from "react";
import type { JSX } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Editor from "@monaco-editor/react";
import { read, write } from "@/lib/file";
import { resolveLanguage } from "./language";
import { buildMonacoTheme } from "./monaco-theme";
import { Failure } from "@/components/Failure";
import { Button } from "@/components/ui/button";

interface MonacoViewerProps {
  path: string;
}

/** The Monaco-backed text viewer and editor for one selected file. */
export function MonacoViewer({ path }: MonacoViewerProps): JSX.Element {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<string | null>(null);

  const content = useQuery({
    queryKey: ["file-content", path],
    queryFn: () => read(path),
  });

  // A dirty draft belongs to the file that was open when it was typed; the
  // next file opened starts clean.
  useEffect(() => {
    setDraft(null);
  }, [path]);

  const save = useMutation({
    mutationFn: (text: string) => write(path, text),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["file-content", path] });
      setDraft(null);
    },
  });

  if (content.isLoading) {
    return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Loading…</div>;
  }
  if (content.error) {
    return (
      <div className="p-4">
        <Failure error={content.error} />
      </div>
    );
  }

  const file = content.data;
  if (!file) return <></>;

  if (file.base64 !== undefined) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-1 text-sm text-muted-foreground">
        <p className="font-medium text-foreground">{path}</p>
        <p>Not editable as text ({file.mediaType}).</p>
      </div>
    );
  }

  const dirty = draft !== null && draft !== file.text;

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b px-3 py-1.5">
        <span className="truncate text-sm font-medium">
          {path}
          {dirty ? " •" : ""}
          {file.truncated ? " (truncated)" : ""}
        </span>
        <Button
          size="sm"
          disabled={!dirty || save.isPending}
          onClick={() => draft !== null && save.mutate(draft)}
        >
          {save.isPending ? "Saving…" : "Save"}
        </Button>
      </div>
      {save.error && (
        <div className="px-3 py-1.5">
          <Failure error={save.error} />
        </div>
      )}
      <div className="min-h-0 flex-1">
        <Editor
          key={path}
          language={resolveLanguage(path)}
          value={draft ?? file.text ?? ""}
          onChange={(value) => setDraft(value ?? "")}
          beforeMount={(monaco) => {
            const built = buildMonacoTheme();
            monaco.editor.defineTheme(built.name, built.theme);
          }}
          onMount={(editor, monaco) => {
            const built = buildMonacoTheme();
            monaco.editor.setTheme(built.name);
            editor.updateOptions({ fontFamily: built.fontFamily, fontSize: built.fontSize });
          }}
          options={{
            minimap: { enabled: false },
            wordWrap: "on",
            tabSize: 2,
            automaticLayout: true,
            bracketPairColorization: { enabled: true },
            scrollBeyondLastLine: false,
          }}
        />
      </div>
    </div>
  );
}
