import { Theme } from "../../../../interfaces/theme.interfaces";

/**
 * The house theme, matching the Go backend's builtin `aos.yaml`
 * (internal/domain/theme/themes/aos.yaml) — same palette, same id ("aos"),
 * used here only as the pre-fetch fallback that renders before
 * `ThemeStore` loads the real preset from the API. Before this rename, that
 * fallback declared id "fractal": a name the backend has never served (its
 * theme list only ever had "aos" and 37 others), so a person who never
 * touched the theme picker booted on a preset the API would 404 on the
 * first `themes_get("fractal")` call.
 */
export const aosTheme: Theme = {
  name: "AOS",
  id: "aos",
  description: "The default AOS theme",
  author: {
    name: "",
    description: "",
    url: ""
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
