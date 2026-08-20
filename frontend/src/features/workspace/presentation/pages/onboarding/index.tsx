import React from "react";
import { aos } from "@/app/aos";
import { OnboardingForm } from "./components/onboarding-form";

export const OnboardingPage = aos.page("/onboarding")
  .withMetadata({
    title: "Welcome to AOS",
    description: "Get started by creating your first workspace",
  })
  //.use(WorkspacePageMiddleware())
  .withComponent(() => {
    return (
      <OnboardingForm />
    );
  })
  .build();
