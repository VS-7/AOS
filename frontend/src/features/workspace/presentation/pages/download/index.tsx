import * as React from "react";
import { aos } from "@/app/aos";
import {
  Apple,
  ArrowRight,
  Bot,
  Check,
  ChevronRight,
  Copy,
  Cpu,
  Download,
  ExternalLink,
  HardDrive,
  Laptop,
  Layers,
  Monitor,
  Package,
  ShieldCheck,
  Terminal,
  Zap,
} from "lucide-react";

const DEFAULT_CDN_BASE = "https://cdn.tryfractal.co/fractal/desktop/releases";
const DEFAULT_VERSION = "0.1.317";
const DEFAULT_CHANNEL = "alpha";

interface Artifact {
  name: string;
  size: number;
  sha256: string;
  key: string;
  url: string;
}

interface ReleaseIndex {
  product: string;
  version: string;
  channel: string;
  publishedAt: string;
  artifacts: Artifact[];
}

interface DetectedOSInfo {
  os: "mac-arm64" | "mac-x64" | "windows-x64" | "linux-x64";
  label: string;
  shortLabel: string;
  sublabel: string;
  fileExt: string;
  icon: React.ElementType;
}

function detectOS(): DetectedOSInfo {
  if (typeof window === "undefined") {
    return {
      os: "mac-arm64",
      label: "macOS (Apple Silicon)",
      shortLabel: "macOS",
      sublabel: "Apple Silicon M1/M2/M3/M4",
      fileExt: ".dmg",
      icon: Apple,
    };
  }

  const ua = navigator.userAgent.toLowerCase();
  const platform = (navigator.platform || "").toLowerCase();

  const isMac = platform.includes("mac") || ua.includes("macintosh") || ua.includes("mac os");
  const isWin = platform.includes("win") || ua.includes("windows");
  const isLinux = platform.includes("linux") || ua.includes("linux");

  if (isMac) {
    const isArm64 =
      ua.includes("arm64") ||
      ua.includes("aarch64") ||
      (navigator.maxTouchPoints && navigator.maxTouchPoints > 2);

    if (isArm64) {
      return {
        os: "mac-arm64",
        label: "macOS (Apple Silicon)",
        shortLabel: "macOS (Apple Silicon)",
        sublabel: "For M1, M2, M3 & M4 Macs",
        fileExt: ".dmg",
        icon: Apple,
      };
    }

    return {
      os: "mac-x64",
      label: "macOS (Intel)",
      shortLabel: "macOS (Intel)",
      sublabel: "For Intel Macs",
      fileExt: ".dmg",
      icon: Apple,
    };
  }

  if (isWin) {
    return {
      os: "windows-x64",
      label: "Windows (64-bit)",
      shortLabel: "Windows",
      sublabel: "For Windows 10 & 11",
      fileExt: ".exe",
      icon: Monitor,
    };
  }

  if (isLinux) {
    return {
      os: "linux-x64",
      label: "Linux (64-bit)",
      shortLabel: "Linux",
      sublabel: "AppImage & Debian / Ubuntu .deb",
      fileExt: ".AppImage",
      icon: Laptop,
    };
  }

  return {
    os: "mac-arm64",
    label: "macOS (Apple Silicon)",
    shortLabel: "macOS",
    sublabel: "Apple Silicon",
    fileExt: ".dmg",
    icon: Apple,
  };
}

export const DownloadPage = aos
  .page("/download")
  .withMetadata({
    title: "Download Fractal Desktop — Your Intelligent Workspace",
    description:
      "Download Fractal Desktop for macOS, Windows, and Linux. Your workspace that thinks, remembers, and works with you.",
  })
  .withComponent(() => {
    const [detected, setDetected] = React.useState<DetectedOSInfo>({
      os: "mac-arm64",
      label: "macOS (Apple Silicon)",
      shortLabel: "macOS",
      sublabel: "Apple Silicon M1/M2/M3/M4",
      fileExt: ".dmg",
      icon: Apple,
    });

    const [release, setRelease] = React.useState<ReleaseIndex | null>(null);
    const [copiedCli, setCopiedCli] = React.useState(false);
    const [activeTab, setActiveTab] = React.useState<"all" | "mac" | "win" | "linux">("all");

    React.useEffect(() => {
      setDetected(detectOS());

      async function fetchLatestRelease() {
        try {
          const res = await fetch(
            `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/latest/release.json`,
            { cache: "no-store" }
          );
          if (res.ok) {
            const data: ReleaseIndex = await res.json();
            setRelease(data);
          }
        } catch {
          // Fallback if network fails
        }
      }

      void fetchLatestRelease();
    }, []);

    const version = release?.version || DEFAULT_VERSION;

    const getArtifactUrl = React.useCallback(
      (platformKey: string, extension: string) => {
        if (release && release.artifacts?.length) {
          const match = release.artifacts.find(
            (a) =>
              a.key.includes(platformKey) &&
              a.name.toLowerCase().endsWith(extension.toLowerCase())
          );
          if (match) return match.url;
        }

        if (platformKey === "mac-arm64") {
          return `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/${version}/mac-arm64/Fractal-${version}-arm64.dmg`;
        }
        if (platformKey === "mac-x64") {
          return `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/${version}/mac-x64/Fractal-${version}-x64.dmg`;
        }
        if (platformKey === "windows-x64") {
          return `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/${version}/windows-x64/Fractal-Setup-${version}.exe`;
        }
        if (platformKey === "linux") {
          return `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/${version}/linux/Fractal-${version}.AppImage`;
        }

        return `${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/${version}/mac-arm64/Fractal-${version}-arm64.dmg`;
      },
      [release, version]
    );

    const primaryDownloadUrl = getArtifactUrl(detected.os, detected.fileExt);

    const handleCopyCli = () => {
      const cmd = "npm install -g @fractal-os/cli@latest && fractal gateway start";
      void navigator.clipboard.writeText(cmd);
      setCopiedCli(true);
      setTimeout(() => setCopiedCli(false), 2500);
    };

    return (
      <main className="min-h-screen bg-background text-foreground">
        {/* 1. Hero Header */}
        <section className="border-b border-border/50">
          <div className="mx-auto max-w-[1200px] px-6 lg:px-8 py-16 md:py-24">
            <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
              Desktop App
            </p>
            <h1 className="mt-3 max-w-[18ch] text-balance text-[36px] font-medium leading-[1.1] tracking-[-0.03em] text-foreground md:text-[52px] md:leading-[1.08]">
              Fractal Desktop for{" "}
              <span className="font-serif italic font-normal">
                macOS, Windows &amp; Linux
              </span>
            </h1>
            <p className="mt-5 max-w-xl text-[15px] leading-6 text-muted-foreground md:text-[16px]">
              Your intelligent workspace that remembers context, organizes your daily tasks, and works alongside you everywhere.
            </p>
          </div>
        </section>

        {/* 2. Detected System & CLI Options */}
        <section className="border-b border-border/50">
          <div className="mx-auto max-w-[1200px] px-6 lg:px-8 grid gap-0 md:grid-cols-2 md:divide-x md:divide-border/50">
            <div className="border-b border-border/50 py-12 md:border-b-0 md:pr-12 md:py-16">
              <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                Recommended for your computer
              </p>
              <h2 className="mt-3 text-[24px] font-medium tracking-[-0.02em] text-foreground md:text-[28px]">
                {detected.label}
              </h2>
              <p className="mt-3 max-w-md text-[15px] leading-6 text-muted-foreground">
                {detected.sublabel} · Free during early access (v{version})
              </p>

              <div className="mt-8 flex flex-wrap items-center gap-3">
                <a
                  href={primaryDownloadUrl}
                  download
                  className="inline-flex h-10 items-center gap-2 rounded-full bg-primary px-5 text-[13px] font-medium text-primary-foreground transition-opacity hover:opacity-90"
                >
                  <Download className="size-3.5" />
                  Download for {detected.shortLabel}
                </a>
                <a
                  href="#all-builds"
                  className="inline-flex h-10 items-center gap-2 rounded-full border border-border bg-background px-4 text-[13px] font-medium text-foreground transition-colors hover:bg-muted"
                >
                  All versions
                  <ArrowRight className="size-3.5" />
                </a>
              </div>
            </div>

            <div className="py-12 md:pl-12 md:py-16">
              <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                Command Line Interface
              </p>
              <h2 className="mt-3 text-[24px] font-medium tracking-[-0.02em] text-foreground md:text-[28px]">
                Terminal installation
              </h2>
              <p className="mt-3 max-w-md text-[15px] leading-6 text-muted-foreground">
                For advanced users who prefer working in the terminal, Fractal is also available as a global command-line tool.
              </p>

              <div className="mt-6 flex items-center justify-between rounded-md border border-border/60 bg-muted/30 p-3 max-w-md font-mono text-[13px]">
                <span className="text-foreground">$ npm install -g @fractal-os/cli@latest && fractal gateway start</span>
                <button
                  type="button"
                  onClick={handleCopyCli}
                  className="flex items-center gap-1.5 rounded border border-border bg-background px-2.5 py-1 text-[12px] font-sans font-medium text-muted-foreground hover:text-foreground transition-colors"
                >
                  {copiedCli ? (
                    <>
                      <Check className="size-3 text-emerald-500" />
                      <span className="text-emerald-500">Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy className="size-3" />
                      <span>Copy</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </section>

        {/* 3. All Builds Matrix */}
        <section id="all-builds" className="border-b border-border/50 py-16 md:py-20">
          <div className="mx-auto max-w-[1200px] px-6 lg:px-8">
            <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 mb-8">
              <div>
                <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                  All Downloads
                </p>
                <h2 className="mt-2 text-[28px] font-medium leading-9 text-foreground">
                  Available Installers
                </h2>
              </div>

              <div className="flex items-center gap-1 rounded-full border border-border/60 bg-muted/30 p-1 text-[12px]">
                {(["all", "mac", "win", "linux"] as const).map((tab) => (
                  <button
                    key={tab}
                    type="button"
                    onClick={() => setActiveTab(tab)}
                    className={`rounded-full px-3 py-1 capitalize transition-colors ${
                      activeTab === tab
                        ? "bg-background text-foreground font-medium shadow-sm"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {tab === "all" ? "All Platforms" : tab === "mac" ? "macOS" : tab === "win" ? "Windows" : "Linux"}
                  </button>
                ))}
              </div>
            </div>

            <div className="grid gap-px overflow-hidden rounded-md border border-border/50 bg-border/50 sm:grid-cols-2">
              {[
                {
                  platform: "mac",
                  osKey: "mac-arm64",
                  ext: ".dmg",
                  icon: Apple,
                  title: "macOS (Apple Silicon)",
                  arch: "M1 / M2 / M3 / M4 Macs",
                  format: "DMG Installer",
                  desc: "Recommended installer for Apple Silicon Macs.",
                },
                {
                  platform: "mac",
                  osKey: "mac-x64",
                  ext: ".dmg",
                  icon: Apple,
                  title: "macOS (Intel)",
                  arch: "Intel Macs",
                  format: "DMG Installer",
                  desc: "Installer for Intel-based Mac computers.",
                },
                {
                  platform: "win",
                  osKey: "windows-x64",
                  ext: ".exe",
                  icon: Monitor,
                  title: "Windows Setup",
                  arch: "Windows 10 & 11",
                  format: "EXE Setup",
                  desc: "Standard installer for Windows PCs.",
                },
                {
                  platform: "linux",
                  osKey: "linux",
                  ext: ".AppImage",
                  icon: Laptop,
                  title: "Linux AppImage",
                  arch: "64-bit Linux",
                  format: "AppImage",
                  desc: "Standalone binary for all Linux distributions.",
                },
                {
                  platform: "linux",
                  osKey: "linux",
                  ext: ".deb",
                  icon: Laptop,
                  title: "Linux Debian / Ubuntu",
                  arch: "Debian & Ubuntu",
                  format: "DEB Package",
                  desc: "Package installer for Debian-based distros.",
                },
              ]
                .filter((item) => activeTab === "all" || activeTab === item.platform)
                .map((item) => {
                  const ItemIcon = item.icon;
                  const downloadUrl = getArtifactUrl(item.osKey, item.ext);

                  return (
                    <div
                      key={item.title + item.ext}
                      className="flex flex-col justify-between gap-4 bg-background p-6 text-left transition-colors hover:bg-muted/40"
                    >
                      <div className="flex items-start gap-4">
                        <span className="flex size-9 items-center justify-center rounded-md border border-border bg-muted/40 shrink-0">
                          <ItemIcon className="size-4 text-muted-foreground" />
                        </span>
                        <div>
                          <div className="flex items-center gap-2">
                            <h3 className="text-[15px] font-medium text-foreground">
                              {item.title}
                            </h3>
                            <span className="rounded border border-border/60 bg-muted/50 px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground">
                              {item.format}
                            </span>
                          </div>
                          <p className="mt-1 text-[13px] leading-5 text-muted-foreground">
                            {item.desc}
                          </p>
                          <div className="mt-2 flex items-center gap-2 text-[11px] font-mono text-muted-foreground/70">
                            <span>{item.arch}</span>
                            <span>·</span>
                            <span>v{version}</span>
                          </div>
                        </div>
                      </div>

                      <div className="pt-2">
                        <a
                          href={downloadUrl}
                          download
                          className="inline-flex h-9 items-center gap-2 rounded-full border border-border bg-background px-4 text-[12px] font-medium text-foreground transition-colors hover:bg-muted"
                        >
                          <Download className="size-3.5" />
                          Download {item.ext}
                        </a>
                      </div>
                    </div>
                  );
                })}
            </div>
          </div>
        </section>

        {/* 4. Why Fractal */}
        <section className="border-b border-border/50 py-16 md:py-20">
          <div className="mx-auto max-w-[1200px] px-6 lg:px-8">
            <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
              Why Fractal
            </p>
            <h2 className="mt-2 text-[28px] font-medium leading-9 text-foreground">
              A workspace designed around how you think
            </h2>

            <div className="mt-8 grid gap-px overflow-hidden rounded-md border border-border/50 bg-border/50 sm:grid-cols-3">
              {[
                {
                  icon: Cpu,
                  title: "Never Starts Over",
                  description:
                    "Fractal remembers past decisions, project notes, and instructions so you don't have to repeat yourself.",
                },
                {
                  icon: Zap,
                  title: "Fast & Private",
                  description:
                    "Runs smoothly on your computer. Your files, documents, and data stay under your control.",
                },
                {
                  icon: Bot,
                  title: "AI Companions",
                  description:
                    "Collaborate with intelligent assistants configured for your specific projects, writing, and tasks.",
                },
                {
                  icon: ShieldCheck,
                  title: "Safe & Secure",
                  description:
                    "Built with clear privacy boundaries so automated tools operate safely within your workspace.",
                },
                {
                  icon: Layers,
                  title: "Custom Dashboards",
                  description:
                    "Organize tasks, goals, and notes into clean, adaptive views that match how you like to work.",
                },
                {
                  icon: Package,
                  title: "App Integrations",
                  description:
                    "Connect your favorite tools and automation playbooks to streamline your daily routines.",
                },
              ].map((item) => {
                const ItemIcon = item.icon;
                return (
                  <div
                    key={item.title}
                    className="flex gap-4 bg-background p-6 text-left transition-colors hover:bg-muted/40"
                  >
                    <span className="flex size-9 items-center justify-center rounded-md border border-border bg-muted/40 shrink-0">
                      <ItemIcon className="size-4 text-muted-foreground" />
                    </span>
                    <div>
                      <h3 className="text-[15px] font-medium text-foreground">
                        {item.title}
                      </h3>
                      <p className="mt-1 text-[13px] leading-5 text-muted-foreground">
                        {item.description}
                      </p>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </section>

        {/* 5. System Specs & Security */}
        <section className="border-b border-border/50 py-16 md:py-20">
          <div className="mx-auto max-w-[1200px] px-6 lg:px-8 grid gap-0 md:grid-cols-2 md:divide-x md:divide-border/50">
            <div className="border-b border-border/50 pb-10 md:border-b-0 md:pr-12 md:pb-0">
              <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                Compatibility
              </p>
              <h2 className="mt-2 text-[24px] font-medium tracking-[-0.02em] text-foreground">
                System Specifications
              </h2>
              <ul className="mt-4 space-y-3 text-[14px] text-muted-foreground">
                <li className="flex items-center gap-2">
                  <ChevronRight className="size-3.5 text-foreground" />
                  <span><strong>macOS:</strong> 12.0 (Monterey) or later</span>
                </li>
                <li className="flex items-center gap-2">
                  <ChevronRight className="size-3.5 text-foreground" />
                  <span><strong>Windows:</strong> Windows 10 or Windows 11 (64-bit)</span>
                </li>
                <li className="flex items-center gap-2">
                  <ChevronRight className="size-3.5 text-foreground" />
                  <span><strong>Linux:</strong> Ubuntu 20.04+, Debian 11+ or compatible</span>
                </li>
                <li className="flex items-center gap-2">
                  <ChevronRight className="size-3.5 text-foreground" />
                  <span><strong>Hardware:</strong> 4 GB RAM minimum</span>
                </li>
              </ul>
            </div>

            <div className="pt-10 md:pl-12 md:pt-0">
              <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-muted-foreground">
                Security
              </p>
              <h2 className="mt-2 text-[24px] font-medium tracking-[-0.02em] text-foreground">
                Verified &amp; Secure
              </h2>
              <p className="mt-4 text-[14px] leading-6 text-muted-foreground">
                Every release is compiled automatically through secure release pipelines and served directly from our official CDN with verified SHA-256 integrity checksums.
              </p>
              <div className="mt-6">
                <a
                  href={`${DEFAULT_CDN_BASE}/${DEFAULT_CHANNEL}/latest/release.json`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 text-[13px] font-medium text-foreground hover:underline underline-offset-4"
                >
                  View Live Release Index
                  <ExternalLink className="size-3.5" />
                </a>
              </div>
            </div>
          </div>
        </section>
      </main>
    );
  })
  .build();
