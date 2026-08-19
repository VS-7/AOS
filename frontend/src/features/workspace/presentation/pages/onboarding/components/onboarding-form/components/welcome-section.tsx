"use client";

import { AnimatePresence, motion } from "framer-motion";
import { useEffect, useState } from "react";
import chatDmWithAgentScreenshot from "@/public/screenshots/chat-dm-with-agent.png";
import fileExplorerScreenshot from "@/public/screenshots/file-explorer.png";
import taskDetailsScreenshot from "@/public/screenshots/task-details.png";
import taskListScreenshot from "@/public/screenshots/task-list.png";

interface CarouselImageAnimation {
  focalPoint: {
    x: number;
    y: number;
  };
  initialZoom: number;
  targetZoom: number;
  delayMs: number;
  durationMs: number;
  holdMs: number;
}

interface CarouselImage {
  src: string;
  alt: string;
  animation: CarouselImageAnimation;
}

const INTRO_DURATION_MS = 4000;
const FADE_TRANSITION = {
  duration: 0.6,
  ease: [0.22, 1, 0.36, 1] as const,
};

const screenshots: CarouselImage[] = [
  {
    src: chatDmWithAgentScreenshot,
    alt: "Talk with your Copilot to turn intent into action, keep context alive across sessions, and coordinate specialist agents inside your workspace.",
    animation: {
      focalPoint: { x: 0, y: 18 },
      initialZoom: 1,
      targetZoom: 2,
      delayMs: 2000,
      durationMs: 3000,
      holdMs: 3000,
    },
  },
  {
    src: fileExplorerScreenshot,
    alt: "Explore your files, decisions, and references in an adaptive space that connects what you do to the real context of the work.",
    animation: {
      focalPoint: { x: 0, y: 0 },
      initialZoom: 1,
      targetZoom: 2,
      delayMs: 2000,
      durationMs: 3000,
      holdMs: 3000,
    },
  },
  {
    src: taskDetailsScreenshot,
    alt: "Open tasks with clarity, track plans, status, and next steps, and let Fractal help you orchestrate execution end to end.",
    animation: {
      focalPoint: { x: 90, y: 0 },
      initialZoom: 1,
      targetZoom: 2,
      delayMs: 2000,
      durationMs: 3000,
      holdMs: 3000,
    },
  },
  {
    src: taskListScreenshot,
    alt: "See your work as a continuous flow that helps you prioritize, track, and move tasks forward with continuity and supervision.",
    animation: {
      focalPoint: { x: 50, y: 28 },
      initialZoom: 1,
      targetZoom: 1.1,
      delayMs: 2000,
      durationMs: 3000,
      holdMs: 3000,
    },
  },
];

export function WelcomeSection() {
  const [currentIndex, setCurrentIndex] = useState(0);
  const [showIntro, setShowIntro] = useState(true);

  const current = screenshots[currentIndex];
  const currentSlideDuration =
    current.animation.delayMs +
    current.animation.durationMs +
    current.animation.holdMs;

  useEffect(() => {
    const introTimeout = window.setTimeout(() => {
      setShowIntro(false);
    }, INTRO_DURATION_MS);

    return () => window.clearTimeout(introTimeout);
  }, []);

  const goToSlide = (index: number) => {
    setCurrentIndex(index);
  };

  useEffect(() => {
    if (showIntro) {
      return;
    }

    const slideTimeout = window.setTimeout(() => {
      goToSlide((currentIndex + 1) % screenshots.length);
    }, currentSlideDuration);

    return () => window.clearTimeout(slideTimeout);
  }, [currentIndex, currentSlideDuration, showIntro]);

  return (
    <div className="h-full flex flex-col justify-between overflow-hidden">
      <main className="relative flex-1 w-full overflow-hidden min-h-0 bg-background">
        {screenshots.map((screenshot, index) => {
          const isActive = index === currentIndex;
          const slideDuration =
            screenshot.animation.delayMs +
            screenshot.animation.durationMs +
            screenshot.animation.holdMs;
          const zoomStart = screenshot.animation.delayMs / slideDuration;
          const zoomEnd =
            (screenshot.animation.delayMs + screenshot.animation.durationMs) /
            slideDuration;

          return (
            <motion.img
              key={screenshot.src}
              src={screenshot.src}
              alt={screenshot.alt}
              fetchPriority={index === 0 ? "high" : "auto"}
              className="absolute inset-0 h-full w-full object-cover object-top"
              style={{
                transformOrigin: `${screenshot.animation.focalPoint.x}% ${screenshot.animation.focalPoint.y}%`,
              }}
              initial={false}
              animate={
                isActive
                  ? {
                      opacity: 1,
                      scale: showIntro
                        ? screenshot.animation.initialZoom
                        : [
                            screenshot.animation.initialZoom,
                            screenshot.animation.initialZoom,
                            screenshot.animation.targetZoom,
                            screenshot.animation.targetZoom,
                          ],
                    }
                  : {
                      opacity: 0,
                      scale: screenshot.animation.initialZoom,
                    }
              }
              transition={
                isActive
                  ? {
                      opacity: FADE_TRANSITION,
                      scale: showIntro
                        ? { duration: 0 }
                        : {
                            duration: slideDuration / 1000,
                            ease: "easeInOut" as const,
                            times: [0, zoomStart, zoomEnd, 1],
                          },
                    }
                  : {
                      opacity: FADE_TRANSITION,
                      scale: { duration: 0.3 },
                    }
              }
            />
          );
        })}
      </main>
      <footer className="p-4">
        <AnimatePresence mode="wait">
          {showIntro ? (
            <motion.div
              key="intro-copy"
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -12 }}
              transition={FADE_TRANSITION}
              className="flex max-w-[36rem] mx-auto flex-col items-center gap-2 text-center"
            >
              <h2 className="text-base font-semibold tracking-tight">
                Welcome to Fractal
              </h2>
              <p className="text-base text-muted-foreground">
                A quick look at how Fractal keeps your context alive, adapts to
                your work, and turns what you mean into what gets done.
              </p>
            </motion.div>
          ) : (
            <motion.div
              key={`slide-copy-${currentIndex}`}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -12 }}
              transition={FADE_TRANSITION}
              className="flex flex-col items-center justify-center gap-4"
            >
              <div className="flex items-center gap-2">
                {screenshots.map((_, index) => (
                  <button
                    key={index}
                    onClick={() => goToSlide(index)}
                    className={`h-1.5 rounded-full transition-all duration-300 ${
                      index === currentIndex
                        ? "w-6 bg-foreground"
                        : "w-1.5 bg-foreground/30 hover:bg-foreground/50"
                    }`}
                    aria-label={`Go to slide ${index + 1}`}
                  />
                ))}
              </div>

              <div className="max-w-[36rem] text-center text-base font-medium">
                {current.alt}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </footer>
    </div>
  );
}
