---
name: simplefs
description: "Lightweight file exchange server with HTMX, Tailwind CSS and Go"
colors:
  # Light theme. Every token is redefined for dark mode in web/src/input.css.
  background: "#f9f9fc"
  surface-container-lowest: "#ffffff"
  surface-container-low: "#f3f3f6"
  surface-container: "#eeeef0"
  surface-container-high: "#e8e8ea"
  surface-container-highest: "#e2e2e5"
  surface-raised: "#ffffff"
  on-surface: "#1a1c1e"
  on-surface-variant: "#424752"
  outline: "#727783"
  outline-variant: "#c2c6d4"
  primary: "#00488d"
  primary-container: "#d6e3ff"
  brand: "#00488d"
  brand-hover: "#005fb8"
  on-brand: "#ffffff"
  secondary: "#535f70"
  error: "#ba1a1a"
  error-container: "#ffdad6"
  success: "#059669"
  accent-code: "#155e75"
  accent-media: "#6d28d9"
  accent-audio: "#9d174d"
  accent-archive: "#92400e"
  accent-sheet: "#065f46"
colors-dark:
  background: "#111316"
  surface-container-lowest: "#0d0e10"
  surface-container-low: "#181a1d"
  surface-container: "#1e2024"
  surface-container-high: "#24272c"
  surface-container-highest: "#2e3238"
  surface-raised: "#1e2024"
  on-surface: "#e2e2e5"
  on-surface-variant: "#c2c6d4"
  outline-variant: "#383c44"
  primary: "#a8c8ff"
  brand: "#005fb8"
  brand-hover: "#1a73d2"
  error: "#ffb4ab"
  accent-code: "#22d3ee"
  accent-media: "#a78bfa"
  accent-audio: "#f472b6"
  accent-archive: "#fbbf24"
  accent-sheet: "#34d399"
typography:
  display: { fontFamily: "Inter Variable", fontSize: "2rem", fontWeight: 700, lineHeight: "2.5rem", letterSpacing: "-0.02em" }
  headline: { fontFamily: "Inter Variable", fontSize: "1.5rem", fontWeight: 700, lineHeight: "2rem", letterSpacing: "-0.01em" }
  title: { fontFamily: "Inter Variable", fontSize: "1.125rem", fontWeight: 600, lineHeight: "1.5rem" }
  body: { fontFamily: "Inter Variable", fontSize: "0.875rem", fontWeight: 400, lineHeight: "1.25rem" }
  body-sm: { fontFamily: "Inter Variable", fontSize: "0.8125rem", fontWeight: 400, lineHeight: "1.125rem" }
  label: { fontFamily: "Inter Variable", fontSize: "0.75rem", fontWeight: 500, letterSpacing: "0.04em" }
  mono: { fontFamily: "JetBrains Mono Variable", fontSize: "0.75rem", fontWeight: 400 }
  icon: { family: "Material Symbols Outlined (SVG sprite)", size: "1em of the current font size" }
rounded:
  lg: "8px"
  xl: "12px"
  2xl: "16px"
  3xl: "24px"
  full: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  btn-primary: { backgroundColor: "{colors.brand}", textColor: "{colors.on-brand}", rounded: "{rounded.full}", height: "40px" }
  btn-secondary: { backgroundColor: "{colors.surface-raised}", border: "{colors.outline-variant}", rounded: "{rounded.full}", height: "36px" }
  card: { backgroundColor: "{colors.surface-container-low}", border: "{colors.outline-variant} 60%", rounded: "{rounded.2xl}" }
  modal: { backgroundColor: "{colors.surface-container}", rounded: "{rounded.3xl}" }
  fab: { backgroundColor: "{colors.brand}", textColor: "{colors.on-brand}", rounded: "{rounded.2xl}", size: "56px" }
---

# Design System: simplefs

## Overview

**Creative North Star: "The Ambient Workspace"**

SimpleFS sigue el lenguaje visual de Material Design 3: superficies neutras en capas, azul cobalto para la identidad y las acciones principales, tipografía `Inter` y datos en `JetBrains Mono`. Todo prioriza la utilidad directa: ver qué llegó, enviarlo o descargarlo con el menor número de clics.

**Fuente de verdad:** los tokens viven como variables CSS (canales RGB) en [`web/src/input.css`](./web/src/input.css) y Tailwind los expone en [`tailwind.config.js`](./tailwind.config.js) con un único nombre semántico (`bg-surface-container-low`, `text-on-surface`, `bg-brand`…). **El modo oscuro solo redefine las variables**: las plantillas nunca usan variantes `-light`/`-dark` ni `dark:` para colores.

## Colors

### Superficies (claro / oscuro)
- **background** (`#f9f9fc` / `#111316`): fondo de la aplicación.
- **surface-container-low** (`#f3f3f6` / `#181a1d`): barra lateral, tarjetas de carpetas y archivos, lienzo de vistas previas.
- **surface-container** (`#eeeef0` / `#1e2024`): modales y cabeceras de tabla.
- **surface-container-high / highest**: estados hover, chips y controles secundarios.
- **surface-raised** (`#ffffff` / `#1e2024`): elementos elevados sobre el fondo: cabecera, controles, tarjeta en hover y toasts.
- **on-surface / on-surface-variant**: texto principal y metadatos.
- **outline-variant**: bordes de 1 px y divisores.

### Marca y acento
- **brand** (`#00488d` / `#005fb8`) con **on-brand** blanco: botones primarios, FAB, navegación activa y chips seleccionados. Contraste ≥ 4.5:1 en ambos temas.
- **primary** (`#00488d` / `#a8c8ff`): iconos de acento, enlaces y texto destacado sobre superficies neutras.

### Funcionales
- **error** para acciones destructivas y PDF; **success** para confirmaciones.
- **Acentos por tipo de archivo**, más oscuros en claro y más luminosos en oscuro para que el texto de los badges cumpla ≈4.5:1 sobre su propio tinte: `accent-code` (cian: código), `accent-media` (violeta: imagen y vídeo), `accent-audio` (rosa), `accent-archive` (ámbar: ZIP y carpetas de build), `accent-sheet` (esmeralda: hojas de cálculo y carpetas de documentación).
- El backend no emite colores sueltos. Cada tipo declara un *acento* semántico (`internal/filetype`) que se traduce a clases literales `text-accent-*` / `bg-accent-*/15`. Tailwind escanea también los `.go`, así que esas clases siempre existen en el CSS.

### Named Rules
**The Clarity Contrast Rule.** `brand` se reserva para navegación y acciones clave; los contenedores de datos permanecen neutros para que destaque el contenido.

## Typography

- **Sans:** `Inter Variable` (un solo archivo latin/latin-ext con todos los pesos 100–900, sin negritas sintéticas).
- **Mono:** `JetBrains Mono Variable` (latin) para fechas, tamaños, rutas, código y badges.

| Token | Uso |
|---|---|
| `text-display` 32/40 bold | Reservado para portadas. |
| `text-headline` 24/32 bold | Título de la vista actual. |
| `text-title` 18/24 semibold | Secciones ("Carpetas", "Archivos"), títulos de modal y de vista previa. |
| `text-body` 14/20 | Texto general y nombres en tarjetas. |
| `text-body-sm` 13/18 | Tabla de archivos, breadcrumbs, controles. |
| `text-label` 12 medium +0.04em | Etiquetas, chips y cabeceras de tabla (en mayúsculas). |
| `text-mono` 12 mono | Metadatos tabulares. |

**The Readable Data Rule.** Fechas, tamaños y rutas usan siempre la fuente monoespaciada.

## Iconography

Material Symbols Outlined como **sprite SVG** (`/static/icons.svg`), generado por `scripts/build-icons.mjs` con solo los iconos usados (~75 símbolos, 25 KB). En las plantillas se escribe `{{ icon "nombre" "clases" }}` o `{{ iconFill ... }}`. El tamaño sigue a `font-size` (`text-xl`, `text-[20px]`…) y el color a `currentColor`. Usar un nombre que no exista hace fallar el build.

## Layout

- Barra lateral fija de 256 px desde `md`; en móvil es un panel deslizante con fondo semitransparente.
- Cabecera de 64 px con búsqueda (Ctrl/⌘ K o `/`). Bajo `sm` el selector de idioma pasa a la barra lateral y las acciones de creación al FAB.
- Contenido con ancho máximo de 1280 px (`max-w-7xl`) y secciones separadas por `space-y-6`.
- **Carpetas:** tarjetas en cuadrícula de 1 → 4 columnas (`sm:2 lg:3 xl:4`).
- **Archivos (cuadrícula):** 2 → 6 columnas (`sm:3 lg:4 xl:5 2xl:6`) con miniatura 4:3.
- **Archivos (lista):** Nombre · Ubicación (solo en búsqueda, recientes y papelera) · Modificado · Tamaño · Acciones.

## Elevation & Depth

**The Surface-First Depth Rule.** La jerarquía se construye con tonos de superficie; las sombras quedan para lo que flota.
- `shadow-modal`: modales, toasts y vistas previas.
- `shadow-fab`: FAB (sombra teñida con `brand`).
- `shadow-card`: tarjeta en hover.

## Shapes

`rounded-lg` (8 px) para botones de icono y badges · `rounded-xl` (12 px) para controles, inputs y elementos de navegación · `rounded-2xl` (16 px) para tarjetas, tablas y FAB · `rounded-3xl` (24 px) para modales · `rounded-full` para botones de texto, chips y búsqueda. Los bordes son de 1 px con `outline-variant` (60 % en tarjetas).

## Components

Las clases de componente están en `@layer components` de `input.css`:
- **`btn-primary` / `btn-secondary` / `btn-ghost` / `btn-danger`**: botones de texto en pastilla.
- **`icon-btn` / `icon-btn-danger`**: acciones de 32 px en filas y tarjetas.
- **`control`, `segment`**: selectores (orden, vista, idioma); el estado activo se expresa con `aria-pressed`/`aria-current`, no con clases.
- **`chip`**: filtros por categoría.
- **`nav-item`**: navegación lateral (`aria-current="page"` = activo).
- **`card`, `modal-panel`, `field`, `badge`**.
- **Toasts** (abajo a la derecha, encima del FAB en móvil): éxito, error e info. El servidor los dispara con la cabecera `HX-Trigger`.

## Interaction & Accessibility

- Las acciones de cada archivo aparecen al hacer hover o al enfocar con teclado (`.reveal`). **En pantallas táctiles siempre se ven.**
- Las carpetas son enlaces reales: se pueden abrir con teclado y en una pestaña nueva. Las vistas previas son `<button>`.
- Los modales atrapan el foco, se cierran con Escape o al hacer clic fuera y devuelven el foco al elemento que los abrió.
- Se respeta `prefers-reduced-motion` y las barras de scroll siempre son visibles.
- La vista se refresca sola cuando cambia el contenido (polling ligero de `/api/v1/stamp`).

## Do's and Don'ts

### Do
- **Do** usar solo tokens semánticos; si falta uno, añadirlo en `input.css` (claro y oscuro) y en `tailwind.config.js`.
- **Do** usar la fuente monoespaciada para fechas, tamaños y rutas.
- **Do** cablear las interacciones con `data-action` (delegación en `web/src/app/actions.js`; cada módulo de `web/src/app/` tiene una responsabilidad). La CSP no permite scripts inline.

### Don't
- **Don't** usar colores de la paleta por defecto de Tailwind (`text-blue-600`…), hex sueltos ni variantes `dark:` para colores.
- **Don't** usar bordes gruesos ni sombras duras que compitan con el contenido.
- **Don't** recargar la página completa: navegación, filtros y acciones van por fragmentos HTMX.
