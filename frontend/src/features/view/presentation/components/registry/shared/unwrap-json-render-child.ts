import * as React from "react";

/**
 * Unwraps json-render devtools `display: contents` spans around rendered elements.
 */
export function unwrapJsonRenderChild(
  child: React.ReactNode,
): React.ReactNode {
  if (!React.isValidElement(child)) return child;

  const props = child.props as {
    children?: React.ReactNode;
    style?: React.CSSProperties;
    "data-jr-key"?: string;
  };

  if (
    child.type === "span" &&
    props["data-jr-key"] &&
    props.style?.display === "contents" &&
    props.children
  ) {
    return props.children;
  }

  return child;
}

/**
 * Returns direct json-render children with devtools wrappers removed.
 */
export function listJsonRenderChildren(
  children: React.ReactNode,
): React.ReactElement[] {
  return React.Children.toArray(children)
    .map(unwrapJsonRenderChild)
    .filter(React.isValidElement);
}
