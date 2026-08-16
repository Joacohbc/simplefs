/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
    "./web/static/**/*.js"
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        background: "var(--color-background)",
        surface: "var(--color-surface)",
        "surface-bright": "var(--color-surface-bright)",
        "surface-dim": "var(--color-surface-dim)",
        "surface-container-lowest": "var(--color-surface-container-lowest)",
        "surface-container-low": "var(--color-surface-container-low)",
        "surface-container": "var(--color-surface-container)",
        "surface-container-high": "var(--color-surface-container-high)",
        "surface-container-highest": "var(--color-surface-container-highest)",
        "surface-variant": "var(--color-surface-variant)",
        "on-surface": "var(--color-on-surface)",
        "on-surface-variant": "var(--color-on-surface-variant)",
        "inverse-surface": "var(--color-inverse-surface)",
        "inverse-on-surface": "var(--color-inverse-on-surface)",
        outline: "var(--color-outline)",
        "outline-variant": "var(--color-outline-variant)",

        primary: "var(--color-primary)",
        "primary-container": "var(--color-primary-container)",
        "on-primary": "var(--color-on-primary)",
        "on-primary-container": "var(--color-on-primary-container)",
        "primary-fixed": "var(--color-primary-fixed)",
        "primary-fixed-dim": "var(--color-primary-fixed-dim)",
        "on-primary-fixed": "var(--color-on-primary-fixed)",
        "on-primary-fixed-variant": "var(--color-on-primary-fixed-variant)",

        secondary: "var(--color-secondary)",
        "secondary-container": "var(--color-secondary-container)",
        "on-secondary": "var(--color-on-secondary)",
        "on-secondary-container": "var(--color-on-secondary-container)",

        error: "var(--color-error)",
        "error-container": "var(--color-error-container)",
        "on-error": "var(--color-on-error)",
        "on-error-container": "var(--color-on-error-container)"
      },
      borderRadius: {
        "DEFAULT": "0.25rem",
        "lg": "0.5rem",
        "xl": "0.75rem",
        "2xl": "1rem",
        "3xl": "1.5rem",
        "full": "9999px"
      },
      spacing: {
        "gutter": "24px",
        "margin-desktop": "64px",
        "margin-mobile": "16px",
        "xs": "4px",
        "sm": "8px",
        "md": "16px",
        "lg": "24px",
        "xl": "32px"
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"]
      }
    }
  },
  plugins: [
    require('@tailwindcss/forms')({ strategy: 'class' }),
    require('@tailwindcss/container-queries')
  ]
}
