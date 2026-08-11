# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
Desarrolladores, sysadmins y entusiastas que necesitan un servidor de archivos ultra rápido, ligero y sin dependencias para entornos de desarrollo local, contenedores devcontainer o servidores en redes privadas y laboratorios.

## Product Purpose
Proporcionar una interfaz web moderna, limpia e interactiva para explorar, subir, descargar y previsualizar archivos localmente, empaquetado en un único binario ejecutable de Go con HTMX y cero dependencias de terceros.

## Positioning
Frente a servidores de archivos pesados (Nextcloud, ownCloud) o el servidor por defecto básico (`python -m http.server`), SimpleFS ofrece vistas previas enriquecidas (código con resaltado, Markdown renderizado, PDFs embebidos a pantalla completa, audio/video) y operaciones en tiempo real vía HTMX con una huella de recursos mínima.

## Operating Context
Desarrollo en contenedores Docker/devcontainers, transferencia rápida de archivos entre equipos en red local e inspección rápida de código y documentación Markdown sin necesidad de instalar dependencias externas ni construir bundles JS complejos.

## Capabilities and Constraints
- **Capacidades**: Navegación navegable por carpetas con breadcrumbs, subida múltiple drag-and-drop, vista previa enriquecida (resaltado de sintaxis con Highlight.js, renderizado Markdown con Marked.js, visor PDF integrado, reproductores multimedia HTML5), creación de carpetas, eliminación asíncrona, filtrado y búsqueda instantánea.
- **Restricciones Técnicas**: Cero dependencias Go externas (librería estándar `net/http` + `embed.FS`), binario único autónomo, arquitectura basada en HTMX + CSS Vanilla.

## Brand Commitments
- Nombre: **SimpleFS**
- Identidad: Estética oscura moderna, limpia, minimalista y libre de saturación visual o elementos decorativos innecesarios.

## Evidence on Hand
- Código fuente de servidor Go en `main.go`.
- Plantillas HTML y recursos estáticos embebidos en `templates/` y `static/style.css`.
- Entorno de desarrollo aislado configurado en `.dc_simplefs/`.

## Product Principles
1. **Velocidad y Ligereza ante todo**: Respuestas inmediatas vía HTMX fragments sin recargas de página ni frameworks JS pesados.
2. **Utilidad Directa (Function-Driven)**: Interfaz libre de fricción enfocada en la navegación, inspección y gestión fluida de archivos.
3. **Autocontenido**: Todo el servidor vive en un único ejecutable distribuible de Go.
4. **Vistas Previas Enriquecidas**: El contenido de los archivos se puede inspeccionar directamente con calidad visual superior sin necesidad de descargarlo primero.

## Accessibility & Inclusion
Cumplimiento de contrastes adecuados en tema oscuro, etiquetas semánticas HTML5 e indicadores de carga para operaciones asíncronas.
