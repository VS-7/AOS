import React from "react";
import { FileQuestion } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "@tanstack/react-router";

export function NotFoundComponent() {
  return (
    <div className="flex min-h-full flex-col items-center justify-center p-8 bg-background text-foreground">
      <div className="flex flex-col items-center text-center max-w-sm space-y-4">
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-muted/50">
          <FileQuestion className="h-8 w-8 text-muted-foreground" strokeWidth={1.5} />
        </div>
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">
            Page not found
          </h1>
          <p className="text-sm text-muted-foreground leading-relaxed">
            The page you're looking for doesn't exist or has been moved. Check the URL or navigate back home.
          </p>
        </div>
        <div className="pt-4">
          <Button asChild variant="default" className="shadow-sm">
            <Link to="/">
              Return Home
            </Link>
          </Button>
        </div>
      </div>
    </div>
  );
}
