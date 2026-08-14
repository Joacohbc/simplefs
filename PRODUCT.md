# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
Desarrolladores, sysadmins y usuarios que necesitan un servidor y explorador de archivos ultra rápido, ligero y visualmente pulido para entornos locales, contenedores devcontainer o servidores en redes privadas.

## Product Purpose
Proporcionar una interfaz web interactiva y moderna (Material Design 3) para explorar, subir, descargar, inspeccionar y previsualizar archivos localmente, empaquetado en un único binario ejecutable de Go con HTMX y Tailwind CSS.

## Positioning
Frente a exploradores de archivos pesados o servidores HTTP básicos de consola, SimpleFS combina la portabilidad de un único ejecutable en Go con una experiencia de usuario moderna: soporte de tema claro/oscuro, vistas en lista y cuadrícula, previsualización enriquecida con barra de herramientas (zoom, impresión, copia, visor PDF/código/markdown) y acciones rápidas desde botón flotante (FAB Speed Dial).

## Operating Context
Desarrollo en contenedores Docker/devcontainers, transferencia rápida de archivos en red local e inspección ágil de código fuente, imágenes, PDFs y documentación multimedia en el navegador.

## Capabilities and Constraints
- **Capacidades**:
  - Navegación navegable por carpetas con breadcrumbs dinámicos y conteo de elementos.
  - Vistas conmutables: Cuadrícula (Grid) y Lista Refinada con persistencia en `localStorage`.
  - Soporte para Tema Claro y Tema Oscuro con selector rápido en encabezado y detección de preferencias del sistema.
  - Subida múltiple drag-and-drop global y selector de archivos nativo.
  - Modal de captura y guardado directo desde el portapapeles (texto, código e imágenes con previsualización en vivo).
  - Modal de Detalles del Archivo con metadatos completos (tipo, tamaño, fechas de creación/modificación y ubicación).
  - Visor a pantalla completa con toolbar (Zoom de PDF, copiado de código con syntax highlighting, Markdown con Marked.js, reproductores multimedia HTML5 y función de impresión).
  - Botón de Acción Flotante (FAB) con menú Speed Dial para acciones rápidas.
  - Filtrado y búsqueda instantánea en tiempo real sin recargas de página vía HTMX.
- **Restricciones Técnicas**:
  - Backend en Go con librería estándar (`net/http` + `embed.FS`).
  - Pipeline de assets con `pnpm` y `tailwindcss`.
  - Binario autónomo distribuible.

## Brand Commitments
- Nombre: **simplefs**
- Identidad: Material Design 3 limpio, tonalidad azul primaria (`#00488d` / `#005fb8`), superficies sutiles en capas y tipografía `Inter`.

## Evidence on Hand
- Servidor Go en [`main.go`](file:///home/joaco/projects/simplefs/main.go).
- Plantillas HTML en [`templates/`](file:///home/joaco/projects/simplefs/templates).
- Pipeline y configuración Tailwind en [`package.json`](file:///home/joaco/projects/simplefs/package.json) y [`tailwind.config.js`](file:///home/joaco/projects/simplefs/tailwind.config.js).
- Diseños de Stitch en [`stitch_designs/`](file:///home/joaco/projects/simplefs/stitch_designs).

## Product Principles
1. **Velocidad y Cero Recargas**: Navegación y operaciones asíncronas inmediatas mediante HTMX.
2. **Utilidad Directa (Function-Driven)**: Información accesible con un solo clic, sin menús anidados innecesarios ni pasos redundantes.
3. **Autocontenido y Portabilidad**: Todo el frontend compilado e incrustado en un único binario Go.
4. **Vistas Previas Enriquecidas de Primer Nivel**: Inspección directa de cualquier formato habitual (código, imágenes, PDF, markdown, audio/video).
5. **Consistencia Visual y Accesibilidad**: Temas Claro y Oscuro contrastados, adaptables y con soporte para múltiples tamaños de pantalla.

## Accessibility & Inclusion
Contraste accesible según estándares WCAG, soporte semántico HTML5, navegación interactiva mediante teclado y etiquetas de estado.
