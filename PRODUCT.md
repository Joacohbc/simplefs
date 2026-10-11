# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
Personas y agentes de IA que necesitan intercambiar archivos de cualquier tipo entre máquinas (un agente en un PC remoto y su usuario, servidores en una VPN, devcontainers), además de desarrolladores y sysadmins que quieren un explorador web rápido sobre una carpeta.

## Product Purpose
Ofrecer el camino más corto para pasar archivos entre una persona y un agente (o entre dos máquinas) a través de una VPN: un único binario de Go o una imagen Docker de ~11 MB, con una interfaz web Material Design 3 para humanos y una API HTTP sencilla para agentes y scripts.

## Positioning
Frente a Samba, SFTP, Nextcloud o servidores HTTP de consola, SimpleFS no necesita configuración (el token se genera solo), es seguro por defecto en redes privadas y está pensado para automatizarse con `curl`. Aun así mantiene una experiencia visual cuidada: temas claro y oscuro, vistas en lista y cuadrícula, y vistas previas enriquecidas.

## Operating Context
Contenedores Docker o binarios en PCs y servidores dentro de una VPN (Tailscale, WireGuard), con agentes que suben resultados a `inbox/` y descargan entradas de `outbox/`, y una persona que revisa todo desde el navegador.

## Capabilities and Constraints
- **Capacidades**:
  - Autenticación por token (total y de solo lectura), sesión por cookie firmada, protección contra DNS rebinding y CSRF, y CSP estricta.
  - API `/api/v1` en JSON: subida en streaming (`PUT`, multipart), descarga con `Range`, listado, `stat` con SHA-256, búsqueda, recientes, carpetas, borrado y detección de cambios.
  - Escritura atómica con política de conflicto (fallar, renombrar o sobrescribir enviando la versión anterior a la papelera).
  - Sin límite de tamaño ni timeouts globales (límite configurable).
  - Navegación por carpetas con breadcrumbs, búsqueda recursiva, vista de recientes y papelera con restauración.
  - Vistas en cuadrícula y lista, temas claro y oscuro, español e inglés.
  - Subida múltiple con arrastrar y soltar y barra de progreso; ZIP con extracción segura; descarga de carpetas como ZIP.
  - Portapapeles: guardar texto o imágenes pegadas como archivo.
  - Vista previa de código (resaltado), Markdown, PDF, imagen (zoom, rotación), audio y vídeo.
  - Actualización automática de la vista cuando llegan archivos nuevos.
  - Botón de acción flotante (FAB) con acciones rápidas, toasts de confirmación y acciones accesibles en táctil y con teclado.
- **Restricciones técnicas**:
  - Backend en Go con la librería estándar (`net/http`, `embed.FS`), sin dependencias externas.
  - Frontend con HTMX y Tailwind compilado con `pnpm`, incrustado en el binario (~0,5 MB de assets).
  - Binarios para Linux, macOS y Windows; imagen `scratch` multi-arquitectura.

## Brand Commitments
- Nombre: **simplefs**
- Identidad: Material Design 3 limpio, azul cobalto (`#00488d` / `#005fb8`), superficies en capas y tipografía `Inter`. Ver [DESIGN.md](./DESIGN.md).

## Evidence on Hand
- Servidor y configuración: [`cmd/simplefs/main.go`](./cmd/simplefs/main.go).
- Lógica: [`internal/`](./internal) (`auth`, `storage`, `handlers`, `middleware`, `assets`).
- Plantillas: [`web/templates/`](./web/templates); estilos y JS: [`web/src/`](./web/src).
- Tokens y configuración de Tailwind: [`web/src/input.css`](./web/src/input.css), [`tailwind.config.js`](./tailwind.config.js).

## Product Principles
1. **Velocidad y Cero Recargas**: Navegación y operaciones asíncronas inmediatas mediante HTMX.
2. **Utilidad Directa (Function-Driven)**: Información accesible con un solo clic, sin menús anidados innecesarios ni pasos redundantes.
3. **Autocontenido y Portabilidad**: Todo el frontend compilado e incrustado en un único binario Go.
4. **Vistas Previas Enriquecidas de Primer Nivel**: Inspección directa de cualquier formato habitual (código, imágenes, PDF, markdown, audio/video).
5. **Consistencia Visual y Accesibilidad**: Temas Claro y Oscuro contrastados, adaptables y con soporte para múltiples tamaños de pantalla.
6. **Seguro por Defecto**: Nada es accesible sin token; las opciones inseguras (sin autenticación, cualquier Host) requieren activarse explícitamente.

## Accessibility & Inclusion
Contraste WCAG AA para el texto en ambos temas (verificado para tokens de superficie, marca y acentos), HTML5 semántico, navegación completa con teclado (enlaces reales, foco atrapado en modales, Escape para cerrar), acciones visibles en táctil, regiones `aria-live` para avisos y respeto de `prefers-reduced-motion`.
