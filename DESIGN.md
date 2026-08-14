---
name: simplefs
description: "High-performance, minimalist file explorer and server with HTMX, Tailwind CSS and Go"
colors:
  primary: "#00488d"
  primary-container: "#005fb8"
  on-primary: "#ffffff"
  on-primary-container: "#cadcff"
  primary-fixed: "#d6e3ff"
  primary-fixed-dim: "#a8c8ff"
  surface: "#f9f9fc"
  surface-container-lowest: "#ffffff"
  surface-container-low: "#f3f3f6"
  surface-container: "#eeeef0"
  surface-container-high: "#e8e8ea"
  surface-container-highest: "#e2e2e5"
  on-surface: "#1a1c1e"
  on-surface-variant: "#424752"
  secondary: "#535f70"
  secondary-container: "#d7e3f8"
  on-secondary-container: "#596576"
  error: "#ba1a1a"
  error-container: "#ffdad6"
  outline: "#727783"
  outline-variant: "#c2c6d4"
typography:
  display:
    fontFamily: "Inter, system-ui, sans-serif"
    fontSize: "2rem"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  headline:
    fontFamily: "Inter, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Inter, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.4
  body:
    fontFamily: "Inter, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Inter, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    letterSpacing: "0.05em"
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
  icon:
    fontFamily: "Material Symbols Outlined"
    fontSize: "1.5rem"
rounded:
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  full: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.full}"
    padding: "10px 20px"
  button-primary-hover:
    backgroundColor: "{colors.primary-container}"
  button-secondary:
    backgroundColor: "{colors.surface-container-high}"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.full}"
    padding: "8px 16px"
  card-folder:
    backgroundColor: "{colors.surface-container-low}"
    rounded: "{rounded.lg}"
    padding: "16px"
  fab-action:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.lg}"
---

# Design System: simplefs

## Overview

**Creative North Star: "The Ambient Workspace"**

SimpleFS adopta el lenguaje visual Material Design 3 con un balance refinado entre sobriedad estructural y claridad interactiva. La interfaz se fundamenta en tonos neutros tonales multicapa, acentos azul cobalto de alta confianza y tipografía `Inter` optimizada para escaneo rápido de datos y archivos.

Toda la experiencia visual prioriza la utilidad directa y la ausencia de fricción: las carpetas se presentan como bloques táctiles superiores, los archivos ofrecen conmutación instantánea entre lista y cuadrícula, y las previsualizaciones ricas (código, Markdown, PDF, imágenes y video) se integran con barras de herramientas funcionales completas.

**Key Characteristics:**
- Superficies tonales adaptativas para modo Claro (`#f9f9fc`) y Oscuro (`#121316`).
- Azul Cobalto Primario (`#00488d` / `#005fb8`) para identidad, jerarquía de acciones y navegación activa.
- Vista dual para archivos: Cuadrícula visual con miniaturas responsivas y Lista Refinada tabular con metadatos.
- Botón de Acción Flotante (FAB `+`) con menú Speed Dial interactivo para subida rápida, creación de carpetas y captura desde portapapeles.
- Vistas previas en vivo enriquecidas con toolbar de herramientas (zoom, copiado de sintaxis, impresión y descarga).

## Colors

El sistema cromático sigue la arquitectura tonal de Material Design 3, asegurando contraste accesible en temas Claro y Oscuro.

### Primary
- **Cobalt Blue** (`#00488d` / Dark: `#a8c8ff`): Utilizado para la identidad de marca, navegación principal y botón de acción flotante (FAB).
- **Primary Container** (`#005fb8` / Dark: `#00468b`): Estados de interacción hover y chips de acción activa.
- **On Primary** (`#ffffff`): Texto e iconografía sobre elementos primarios.

### Neutral
- **Base Surface** (`#f9f9fc` / Dark: `#121316`): Fondo base de la aplicación.
- **Surface Container Low** (`#f3f3f6` / Dark: `#181a1d`): Fondo de tarjetas de carpetas, elementos en lista y contenedores de archivos.
- **Surface Container** (`#eeeef0` / Dark: `#1e2024`): Fondo de modales de configuración y detalles.
- **Surface Container High** (`#e8e8ea` / Dark: `#24272c`): Barra superior y controles secundarios.
- **Surface Container Highest** (`#e2e2e5` / Dark: `#2e3238`): Inputs de búsqueda y estados de hover.
- **Primary Text (On-Surface)** (`#1a1c1e` / Dark: `#e2e2e5`): Texto principal de alto contraste.
- **Secondary Text (On-Surface-Variant)** (`#424752` / Dark: `#c2c6d4`): Metadatos, fechas, tamaños y etiquetas.
- **Outline Variant** (`#c2c6d4` / Dark: `#383c44`): Bordes sutiles y divisores estructurales.

### Functional Accents
- **Error Red** (`#ba1a1a` / Dark: `#ffb4ab`): Acciones destructivas (eliminar) e iconos de PDF.
- **Success Emerald** (`#10b981`): Confirmaciones de copia y estados de éxito.
- **Tech Accents**: Colores semánticos para extensiones (Cyan para Go, Amarillo para JavaScript/TypeScript, Verde para hojas de cálculo, Púrpura para video).

### Named Rules
**The Clarity Contrast Rule.** Las acciones primarias y la navegación usan Cobalt Blue de alto contraste; los contenedores de datos permanecen en tonos neutros para que los archivos destaquen por su contenido.

## Typography

**Display Font:** `Inter, system-ui, -apple-system, sans-serif`
**Body Font:** `Inter, system-ui, -apple-system, sans-serif`
**Mono Font:** `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`

### Hierarchy
- **Display** (Bold 700, 2rem / 32px, -0.02em tracking): Título de marca en el encabezado.
- **Headline** (Bold 700, 1.5rem / 24px, -0.01em tracking): Título de la vista previa y modales de archivo.
- **Title** (SemiBold 600, 1.125rem / 18px): Títulos de sección ("Carpetas", "Archivos").
- **Body** (Regular 400, 0.875rem / 14px, line-height 1.5): Nombres de archivo, texto de lectura y descripciones.
- **Label / Mono** (Medium 500, 0.75rem / 12px, font-mono): Tamaños de archivo, fechas y fragmentos de código.

### Named Rules
**The Readable Data Rule.** Los metadatos de archivos (fechas, pesos y rutas) utilizan siempre alineación tabular o fuente monoespaciada para facilitar la comparación rápida.

## Layout

SimpleFS utiliza una cuadrícula fluida responsiva contenida en un ancho máximo de 1280px (`max-w-7xl`).

- **Ritmo Espacial**: Escala de 8px (4px xs, 8px sm, 16px md, 24px lg, 32px xl).
- **Sección de Carpetas**: Cuadrícula adaptable de 1 a 4 columnas (`grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4`).
- **Sección de Archivos (Grid)**: Cuadrícula visual de 2 a 6 columnas (`grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6`).
- **Sección de Archivos (Lista)**: Tabla responsiva con columnas: Nombre (expansible), Creado (130px), Modificado (130px), Tamaño (90px) y Acciones (110px).

## Elevation & Depth

SimpleFS prioriza la estratificación tonal sobre sombras pesadas, reservando elevación física para elementos flotantes interactivos.

### Shadow Vocabulary
- **Modal Shadow** (`0 25px 50px -12px rgba(0, 0, 0, 0.25)`): Elevación máxima para modales y visor a pantalla completa.
- **FAB Shadow** (`0 20px 25px -5px rgba(0, 72, 141, 0.25)`): Profundidad del botón flotante de acción rápida.
- **Card Hover Shadow** (`0 4px 6px -1px rgba(0, 0, 0, 0.08)`): Elevación sutil al interactuar con carpetas y tarjetas de archivo.

### Named Rules
**The Surface-First Depth Rule.** La jerarquía visual se establece mediante variación tonal de superficies (`surface-container-low` → `surface-container-high`); las sombras se reservan para elementos que flotan sobre el flujo del documento (FAB, modales).

## Shapes

- **Corner Radius**:
  - `sm` (`8px`): Badges, botones de acción en miniatura, pills de extensiones.
  - `md` (`12px`): Inputs, controles de toolbar, contenedores de código.
  - `lg` (`16px`): Tarjetas de carpetas y archivos en vista cuadrícula.
  - `xl` (`24px` / `28px`): Modales de detalles y creación de carpeta.
  - `full` (`9999px`): Botones de acción, pills de navegación y barra de búsqueda.
- **Bordes**: 1px sutil (`outline-variant/30`) para definir bordes de contenedores sin recargar visualmente.

## Components

### Buttons
- **Primary Pill**: Redondeado completo (`rounded-full`), fondo Cobalt Blue (`#00488d`), texto blanco, padding (`10px 20px`).
- **Secondary Pill**: Redondeado completo (`rounded-full`), fondo `surface-container-high`, texto `on-surface`.
- **FAB**: Botón redondeado de 56x56px a 64x64px (`rounded-2xl`), fondo Cobalt Blue con elevación de 20px.

### Folder Cards
- **Forma**: Radio de 16px (`rounded-2xl`), padding de 16px (`p-md`), fondo `surface-container-low`.
- **Icono**: Icono de carpeta de 40x40px en contenedor redondeado primario suave.

### File Cards (Grid)
- **Forma**: Radio de 16px (`rounded-2xl`), padding de 12px (`p-3`).
- **Área de Miniatura**: Contenedor cuadrado con relación de aspecto 1:1, centrado de imagen o icono semántico.

### Modals
- **Detalles del Archivo**: Diálogo de 480px centrado con radio de 24px (`rounded-3xl`), cuadrícula de metadatos en 2 columnas y botón de descarga directa.
- **Visor a Pantalla Completa**: Interfaz completa fija con toolbar superior flotante, visor responsivo de PDF y renderizadores enriquecidos de Markdown y código.

## Do's and Don'ts

### Do:
- **Do** mantener el tema Claro y Oscuro sincronizado en todos los modales y estados interactivos.
- **Do** usar la tipografía monoespaciada para fechas, tamaños en bytes y bloques de código.
- **Do** asegurar que el FAB Speed Dial cierre al hacer clic fuera o al seleccionar una acción.

### Don't:
- **Don't** utilizar bordes gruesos o sombras duras que compitan con el contenido de los archivos.
- **Don't** requerir recargas de página completa; todas las transiciones y filtros deben operar vía fragmentos HTMX.
- **Don't** saturar la pantalla con colores primarios: Cobalt Blue se reserva para navegación y acciones clave.
