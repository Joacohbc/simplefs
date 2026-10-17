/** @type {import('tailwindcss').Config} */
const token = (name) => `rgb(var(--c-${name}) / <alpha-value>)`;

const colorNames = [
  "background",
  "surface-container-lowest",
  "surface-container-low",
  "surface-container",
  "surface-container-high",
  "surface-container-highest",
  "surface-raised",
  "on-surface",
  "on-surface-variant",
  "outline",
  "outline-variant",
  "primary",
  "on-primary",
  "primary-container",
  "on-primary-container",
  "brand",
  "brand-hover",
  "on-brand",
  "secondary",
  "error",
  "on-error",
  "error-container",
  "on-error-container",
  "success",
  "scrim",
  "accent-code",
  "accent-media",
  "accent-audio",
  "accent-archive",
  "accent-sheet",
];

module.exports = {
  content: [
    "./web/templates/**/*.html",
    "./web/src/**/*.js",
    "./internal/**/*.go",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: Object.fromEntries(colorNames.map((name) => [name, token(name)])),
      fontFamily: {
        sans: ["Inter Variable", "Inter", "system-ui", "-apple-system", "sans-serif"],
        mono: ["JetBrains Mono Variable", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
      fontSize: {
        display: ["2rem", { lineHeight: "2.5rem", letterSpacing: "-0.02em", fontWeight: "700" }],
        headline: ["1.5rem", { lineHeight: "2rem", letterSpacing: "-0.01em", fontWeight: "700" }],
        title: ["1.125rem", { lineHeight: "1.5rem", fontWeight: "600" }],
        body: ["0.875rem", { lineHeight: "1.25rem" }],
        "body-sm": ["0.8125rem", { lineHeight: "1.125rem" }],
        label: ["0.75rem", { lineHeight: "1rem", letterSpacing: "0.04em", fontWeight: "500" }],
        mono: ["0.75rem", { lineHeight: "1rem" }],
      },
      boxShadow: {
        modal: "0 25px 50px -12px rgb(0 0 0 / 0.25)",
        fab: "0 20px 25px -5px rgb(var(--c-brand) / 0.30)",
        card: "0 4px 6px -1px rgb(0 0 0 / 0.08)",
      },
    },
  },
  plugins: [
    require("@tailwindcss/forms")({ strategy: "class" }),
  ],
};
