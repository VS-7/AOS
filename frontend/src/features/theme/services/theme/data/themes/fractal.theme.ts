import { FractalTheme } from "../../../../interfaces/theme.interfaces";

export const fractalTheme: FractalTheme = {
  name: "Fractal",
  id: "fractal",
  description: "The default Fractal OS theme",
  author: {
    name: "Fractal",
    description: "Fractal Team",
    url: "https://fractal.ai"
  },
  theme: {
    dark: {
      accent: "#ffffff",
      contrast: 90,
      ink: "#f5f5f5",
      radius: "lg",
      windows: "blur",
      surface: "#0d0b08",
      fonts: {
        code: null,
        ui: "Inter"
      },
      semanticColors: {
        diffAdded: "#4ade80",
        diffRemoved: "#e7000b",
        skill: "#8f8f8f"
      }
    },
    light: {
      accent: "#000000",
      contrast: 60,
      ink: "#0d0b08",
      radius: "lg",
      windows: "blur",
      surface: "#f5f5f5",
      fonts: {
        code: null,
        ui: "Inter"
      },
      semanticColors: {
        diffAdded: "#00a240",
        diffRemoved: "#ba2623",
        skill: "#555555"
      }
    },
  },
};
