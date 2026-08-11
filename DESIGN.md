---
name: SimpleFS
description: Estética de consola moderna con precisión de ingeniería para exploración y previsualización de archivos
colors:
  primary: "#38bdf8"
  primary-hover: "#0284c7"
  bg-primary: "#0b0f19"
  bg-surface: "#151c2c"
  bg-surface-hover: "#1e293b"
  border-color: "#27354a"
  text-primary: "#f8fafc"
  text-secondary: "#a0aec0"
  text-muted: "#64748b"
  danger: "#f43f5e"
  success: "#10b981"
  folder: "#fbbf24"
typography:
  display:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "-0.025em"
  body:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.9rem"
    fontWeight: 400
    lineHeight: 1.5
  code:
    fontFamily: "'Fira Code', Consolas, Monaco, monospace"
    fontSize: "0.875rem"
    lineHeight: 1.6
rounded:
  sm: "6px"
  md: "10px"
  lg: "16px"
spacing:
  sm: "8px"
  md: "16px"
  lg: "24px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "#0b0f19"
    rounded: "{rounded.md}"
    padding: "0.6rem 1.1rem"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "#ffffff"
  button-secondary:
    backgroundColor: "{colors.bg-surface}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.md}"
---

# Design System: SimpleFS

## Overview

**Creative North Star: "The Minimalist Terminal"**

SimpleFS adopta una estética de consola de alto rendimiento con precisión de ingeniería. La interfaz se fundamenta en tonos oscuros profundos (Slate/Obsidian), acentos cian eléctricos y tipografía legible de alta densidad de información. 

Toda la experiencia visual prioriza la utilidad directa y la ausencia de fricción: los contenedores son limpios, las transiciones son instantáneas y micro-interactivas, y el contenido de los archivos toma el protagonismo absoluto.

**Key Characteristics:**
- Fondo azul oscuro profundo (`#0b0f19`) con paneles en capas de contraste (`#151c2c`).
- Acento Cyan brillante (`#38bdf8`) reservado estrictamente para acciones primarias e indicadores de estado.
- Vistas previas en vivo integradas para código, Markdown renderizado, visores PDF a pantalla completa y reproductores HTML5.
- Transiciones fluidas con curva de aceleración personalizada (`cubic-bezier(0.16, 1, 0.3, 1)`).

## Colors

El sistema cromático de SimpleFS está diseñado para minimizar la fatiga visual en sesiones prolongadas de desarrollo, utilizando contraste graduado.

### Primary
- **Cyan Eléctrico** (`#38bdf8`): Utilizado para la identidad de marca, botones principales y estados activos.
- **Deep Blue Accent** (`#0284c7`): Estado de interacción hover para componentes primarios.

### Neutral
- **Deep Obsidian** (`#0b0f19`): Superficie base de la aplicación.
- **Dark Slate Surface** (`#151c2c`): Superficie de tarjetas, modales y tablas.
- **Slate Hover** (`#1e293b`): Filas de tablas y elementos interactivos en estado hover.
- **Subtle Border** (`#27354a`): Líneas divisoras y bordes de definición.
- **Primary Text** (`#f8fafc`): Texto principal de alto contraste.
- **Secondary Text** (`#a0aec0`): Metadatos, etiquetas y subtítulos.
- **Muted Text** (`#64748b`): Texto descriptivo secundario e iconos inactivos.

### Functional Accents
- **Rose Red** (`#f43f5e`): Acciones destructivas o peligrosas (eliminar).
- **Emerald Green** (`#10b981`): Confirmaciones de copia y estados de éxito.
- **Amber Yellow** (`#fbbf24`): Iconografía de carpetas y directorios.

### Named Rules
**The Rarity Accent Rule.** El acento cyan eléctrico se utiliza en ≤10% de la pantalla para mantener su poder de atracción visual.

## Typography

**Display Font:** `system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`
**Body Font:** `system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`
**Mono Font:** `'Fira Code', Consolas, Monaco, monospace`

### Hierarchy
- **Display** (Bold 700, 1.5rem, -0.025em tracking): Título de la aplicación en el encabezado.
- **Headline / Modal Title** (SemiBold 600, 1.15rem): Título de previsualización y modales.
- **Body** (Regular 400, 0.9rem, line-height 1.5): Filas de la tabla de archivos y texto navegable.
- **Code / Technical** (Regular 400, 0.875rem, line-height 1.6): Visor de código fuente y bloques de código en Markdown.
- **Label / Badge** (SemiBold 600, 0.75rem): Etiquetas de tipo MIME y badges de tecnología.

## Layout

SimpleFS utiliza una cuadrícula fluida monolítica en contenedor centrado de ancho máximo de 1200px.

- **Ritmo Espacial**: Escala de 8px (8px sm, 16px md, 24px lg).
- **Contenedores de Archivos**: Tabla responsiva con celda de nombre adaptable y acciones alineadas a la derecha.
- **Modal de Vista Previa**: Ocupa el 94vw y 88vh (máx 1100px) con área flex-1 que expande visores de PDF e imágenes al 100% de la capacidad útil.

## Elevation & Depth

SimpleFS utiliza un modelo de profundidad por capas de color de fondo y sombras difusas de ambientación.

### Shadow Vocabulary
- **Modal Shadow** (`0 20px 40px rgba(0, 0, 0, 0.55)`): Elevación máxima para el backdrop y modal de previsualización.
- **Table Card Shadow** (`0 6px 16px rgba(0, 0, 0, 0.35)`): Profundidad de tarjetas y contenedor de archivos.
- **Accent Glow** (`0 4px 12px rgba(56, 189, 248, 0.25)`): Brillo ambiental para botones primarios e icono de marca.

### Named Rules
**The Flat-By-Default Rule.** Las superficies permanecen planas y limpias al reposo; las sombras elevadas aparecen únicamente en modales o estados de interacción activa.

## Shapes

- **Corner Radius**:
  - `sm` (`6px`): Badges, botones de acción pequeños, tags de metadatos.
  - `md` (`10px`): Botones estándar, inputs, contenedores de código e imágenes.
  - `lg` (`16px`): Tarjeta principal de la tabla, zona de subida drag-and-drop y modales.
- **Bordes**: Bordes sutiles de 1px (`#27354a`) en todas las superficies para mantener definición sin saturar.

## Components

### Buttons
- **Shape**: Redondeado sutil (10px radius).
- **Primary**: Fondo Cyan (`#38bdf8`), Texto Oscuro (`#0b0f19`), Padding (`0.6rem 1.1rem`).
- **Secondary**: Fondo Slate (`#151c2c`), Borde (`#27354a`), Texto Claro (`#f8fafc`).
- **Danger**: Fondo Rose translúcido (`rgba(244, 63, 94, 0.12)`), Texto Rose (`#f43f5e`).

### Dropzone (Zona de Subida)
- **Estilo**: Borde punteado de 2px (`#27354a`), fondo semitransparente (`rgba(21, 28, 44, 0.5)`), radio de 16px.
- **Dragover State**: Borde Cyan (`#38bdf8`), fondo iluminado con brillo ambiental Cyan.

### Modal de Previsualización Enriquecida
- **Header**: Fondo oscuro traslúcido (`rgba(11, 15, 25, 0.6)`), título con icono semántico y botones de acción rápida.
- **Meta Bar**: Barra de datos horizontal con chips estilizados para MIME Type, tamaño y fecha.
- **Visor PDF**: Elemento `iframe` responsivo de 100% de ancho con fondo neutro de lectura.

## Do's and Don'ts

### Do:
- **Do** mantener el acento cyan enfocado únicamente en acciones primarias y estados destacados.
- **Do** usar fuentes monoespaciadas (`Fira Code` / `Consolas`) para fragmentos de código, rutas y metadatos técnicos.
- **Do** asegurar que los visores de previsualización utilicen todo el espacio vertical disponible en el modal.

### Don't:
- **Don't** agregar bordes gruesos decorativos (`border-left > 1px`) en tarjetas o bloques de texto.
- **Don't** incluir gradientes de texto ni elementos ornamentales no funcionales.
- **Don't** ocultar controles de acción detrás de menús anidados cuando pueden ser accesibles con 1 clic.
