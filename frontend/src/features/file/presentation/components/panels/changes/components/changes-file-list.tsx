import type { FileChangeEntry } from "@/features/file/interfaces/file.interfaces";
import type { FileExplorerContext } from "@/features/file/interfaces/file.interfaces";
import type { ChangesPanelPreferences } from "@/features/file/presentation/helpers/changes.helper";
import { ChangesFileItem } from "./changes-file-item";

interface ChangesFileListProps {
  files: FileChangeEntry[];
  explorerContext: FileExplorerContext;
  preferences: ChangesPanelPreferences;
  themeType: "light" | "dark";
  expandedPath: string | null;
  onToggle: (path: string) => void;
}

export function ChangesFileList({
  files,
  explorerContext,
  preferences,
  themeType,
  expandedPath,
  onToggle,
}: ChangesFileListProps) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {files.map((file) => (
        <ChangesFileItem
          key={file.path}
          file={file}
          explorerContext={explorerContext}
          preferences={preferences}
          themeType={themeType}
          expanded={expandedPath === file.path}
          onToggle={() => onToggle(file.path)}
        />
      ))}
    </div>
  );
}
