import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

export type WorkspaceAvatarProps = {
  name: string | undefined;
  color?: string | undefined;
  logo?: string | undefined;
  size?: "default" | "sm" | "lg";
  className?: string;
  fallbackClassName?: string;
};

/**
 * Renders a workspace identity avatar using logo URL when present,
 * otherwise a colored initial fallback.
 */
export function WorkspaceAvatar({
  name,
  color,
  logo,
  size = "default",
  className,
  fallbackClassName,
}: WorkspaceAvatarProps) {
  const initial = name?.[0]?.toUpperCase() || "W";

  return (
    <Avatar size={size} className={cn("rounded-md", className)}>
      {logo ? <AvatarImage src={logo} alt={name || "Workspace"} /> : null}
      <AvatarFallback
        className={cn("text-white rounded-md", fallbackClassName)}
        style={{ backgroundColor: color }}
      >
        {initial}
      </AvatarFallback>
    </Avatar>
  );
}
