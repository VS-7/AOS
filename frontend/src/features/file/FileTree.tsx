import { useState } from "react";
import type { JSX } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, File as FileIcon, Folder } from "lucide-react";
import { tree, type FileNode } from "@/lib/file";
import { Failure } from "@/components/Failure";
import { cn } from "@/lib/utils";

interface FileTreeProps {
  selected: string | null;
  onSelect: (path: string) => void;
}

/** The file explorer's root: the workspace's own top-level listing. */
export function FileTree({ selected, onSelect }: FileTreeProps): JSX.Element {
  return (
    <ul role="tree" aria-label="Files" className="py-1">
      <TreeLevel path="" depth={0} selected={selected} onSelect={onSelect} />
    </ul>
  );
}

/**
 * One directory's children, fetched on demand — the tree loads lazily,
 * directory by directory, rather than pulling the whole workspace tree in
 * one call the way Recursive:true would.
 */
function TreeLevel({
  path,
  depth,
  selected,
  onSelect,
}: {
  path: string;
  depth: number;
  selected: string | null;
  onSelect: (path: string) => void;
}): JSX.Element {
  const query = useQuery({
    queryKey: ["file-tree", path],
    queryFn: () => tree(path, false),
  });

  if (query.isLoading) {
    return <li className="px-3 py-1 text-xs text-muted-foreground">Loading…</li>;
  }
  if (query.error) {
    return (
      <li className="px-2 py-1">
        <Failure error={query.error} />
      </li>
    );
  }

  const nodes = query.data?.nodes ?? [];
  if (nodes.length === 0) {
    return <li className="px-3 py-1 text-xs text-muted-foreground">Empty</li>;
  }

  return (
    <>
      {nodes.map((node) => (
        <TreeNode key={node.path} node={node} depth={depth} selected={selected} onSelect={onSelect} />
      ))}
    </>
  );
}

function TreeNode({
  node,
  depth,
  selected,
  onSelect,
}: {
  node: FileNode;
  depth: number;
  selected: string | null;
  onSelect: (path: string) => void;
}): JSX.Element {
  const [open, setOpen] = useState(false);
  const isSelected = node.path === selected;

  return (
    <li role="treeitem" aria-selected={isSelected} aria-expanded={node.dir ? open : undefined}>
      <button
        type="button"
        className={cn(
          "flex w-full items-center gap-1.5 rounded-sm px-2 py-1 text-left text-sm text-foreground/90 hover:bg-accent",
          isSelected && "bg-accent font-medium",
        )}
        style={{ paddingLeft: `${depth * 14 + 8}px` }}
        onClick={() => (node.dir ? setOpen((o) => !o) : onSelect(node.path))}
      >
        {node.dir ? (
          open ? (
            <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
          ) : (
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />
          )
        ) : (
          <span className="size-3.5 shrink-0" />
        )}
        {node.dir ? (
          <Folder className="size-3.5 shrink-0 text-muted-foreground" />
        ) : (
          <FileIcon className="size-3.5 shrink-0 text-muted-foreground" />
        )}
        <span className="truncate">{node.name}</span>
      </button>
      {node.dir && open && (
        <ul role="group">
          <TreeLevel path={node.path} depth={depth + 1} selected={selected} onSelect={onSelect} />
        </ul>
      )}
    </li>
  );
}
