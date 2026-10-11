<div align="center">

# SimpleFS

**Servidor de archivos ligero para intercambiar archivos de cualquier tipo entre personas, agentes y scripts, en un único binario o en una imagen Docker de ~11 MB.**

</div>

SimpleFS es un explorador web y una API HTTP sobre una carpeta. Está pensado para un caso concreto: **un agente (o cualquier máquina) en otro PC y tú, intercambiando archivos rápido a través de una VPN** (Tailscale, WireGuard…), sin montar Samba, SFTP ni servicios en la nube.

- **Seguro por defecto**: token de acceso obligatorio (se genera solo), token opcional de solo lectura, protección contra DNS rebinding y CSRF, CSP estricta.
- **Pensado para agentes**: API `/api/v1` en JSON, subidas en streaming con `curl -T`, checksum SHA-256, códigos HTTP claros y control de conflictos.
- **Transferencias grandes**: sin límite de tamaño ni timeouts globales (configurable), escritura atómica (nunca verás un archivo a medias) y descargas con `Range`.
- **Interfaz cómoda**: claro/oscuro, lista/cuadrícula, arrastrar y soltar, pegar desde el portapapeles, vista previa de código, Markdown, PDF, imagen, audio y vídeo, recientes, búsqueda recursiva y actualización automática cuando llega un archivo.
- **Portable**: binarios para Linux, macOS y Windows (amd64/arm64) e imagen multi-arquitectura `scratch`.

## 🚀 Inicio rápido con Docker

```bash
mkdir -p uploads
docker run -d --name simplefs --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  --user "$(id -u):$(id -g)" \
  -v "$(pwd)/uploads:/data" \
  ghcr.io/joacohbc/simplefs:latest

docker logs simplefs              # muestra el token generado en el primer arranque
docker exec simplefs /simplefs token   # o pídelo en cualquier momento
```

Abre <http://localhost:8080> e introduce el token.

Con Docker Compose (copia `.env.example` a `.env` y ajústalo):

```bash
cp .env.example .env
mkdir -p uploads
docker compose up -d
```

Más detalles en la [guía de Docker](./DOCKER.md).

## 🤝 Agente ↔ tú a través de una VPN

1. **Publica SimpleFS solo en la VPN.** En `.env`, pon en `BIND_IP` la IP de esta máquina dentro de la VPN (p. ej. la `100.x.y.z` de Tailscale). No uses `0.0.0.0`: Docker se salta firewalls como `ufw`.
2. **Fija un token** (`SIMPLEFS_TOKEN=$(openssl rand -hex 24)`) y, si el agente solo debe descargar, dale un `SIMPLEFS_READ_TOKEN`.
3. **Si accedes por nombre** (MagicDNS, `pc.tailnet.ts.net`), añádelo a `ALLOWED_HOSTS`. Las IP y `localhost` siempre se aceptan.
4. **Usa una convención de carpetas**: `inbox/` para lo que te envía el agente y `outbox/` para lo que le envías tú. Ambas tienen icono propio en la interfaz.
5. Opcional: con Tailscale, `tailscale serve --bg 8080` te da HTTPS con certificado válido.

En el PC del agente:

```bash
export SIMPLEFS_URL=http://100.64.0.10:8080
export SIMPLEFS_TOKEN=...

# Enviar un archivo (crea las carpetas que falten; 409 si ya existe)
curl -fsS -T informe.pdf -H "Authorization: Bearer $SIMPLEFS_TOKEN" "$SIMPLEFS_URL/api/v1/files/inbox/informe.pdf"

# Reemplazarlo (la versión anterior va a la papelera)
curl -fsS -T informe.pdf -H "Authorization: Bearer $SIMPLEFS_TOKEN" "$SIMPLEFS_URL/api/v1/files/inbox/informe.pdf?overwrite=true"

# Ver qué hay para él y descargarlo
curl -fsS -H "Authorization: Bearer $SIMPLEFS_TOKEN" "$SIMPLEFS_URL/api/v1/files/outbox/"
curl -fsSLO -H "Authorization: Bearer $SIMPLEFS_TOKEN" "$SIMPLEFS_URL/api/v1/files/outbox/datos.zip"
```

La interfaz incluye **"Conectar agente"** con estos comandos listos para copiar, y cada archivo tiene una acción **"Copiar comando curl"**.

## 📡 API `/api/v1`

Todas las llamadas requieren `Authorization: Bearer <token>`. Las respuestas son JSON (`{"error": "..."}` en caso de error).

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/api/v1/whoami` | Rol del token (`write`/`read`) y límites. |
| `GET` | `/api/v1/files/{ruta}` | Descarga un archivo (con `Range`). Si es carpeta, lista su contenido en JSON. |
| `GET` | `/api/v1/files/{ruta}?stat&checksum=1` | Metadatos y SHA-256. |
| `GET` | `/api/v1/files/{carpeta}?archive=zip` | Descarga la carpeta como ZIP. |
| `PUT` | `/api/v1/files/{ruta}` | Sube el cuerpo de la petición. `?overwrite=true`, `?conflict=fail\|rename\|overwrite` (por defecto `fail` → 409), `?parents=false`. |
| `POST` | `/api/v1/files/{carpeta}` | Sube uno o varios archivos `multipart/form-data` (`curl -F f=@a -F g=@b`). Conflictos: `rename` por defecto. |
| `DELETE` | `/api/v1/files/{ruta}` | Mueve a la papelera; `?permanent=true` borra definitivamente. |
| `POST` | `/api/v1/folders/{ruta}` | Crea la carpeta y sus padres. |
| `GET` | `/api/v1/recent?limit=50` | Últimos archivos modificados en todo el almacenamiento. |
| `GET` | `/api/v1/search?q=texto&path=carpeta` | Búsqueda recursiva por nombre. |
| `GET` | `/api/v1/stamp?scope=dir\|recent\|trash&path=` | Huella barata para detectar cambios por polling. |
| `GET` | `/healthz` | Comprobación de salud (pública). |

Las subidas devuelven `{"name","path","size","sha256","overwritten","renamed"}`, así el agente puede verificar la integridad.

## 🛠️ Desarrollo local

Requisitos: **Go 1.26+**, **Node.js 24+** y **pnpm 10** (`corepack enable`).

```bash
pnpm install
pnpm run build          # iconos, JS, CSS y binario
./simplefs -d ./uploads # el token se imprime en el primer arranque
```

`pnpm run dev:css` recompila Tailwind en caliente. Los tests: `go test ./...`.

### Opciones

| Parámetro | Variable | Por defecto | Descripción |
|---|---|---|---|
| `-p`, `--port` | `PORT` | `8080` | Puerto HTTP. |
| `-b`, `--bind` | `BIND` | todas | Dirección de escucha (`127.0.0.1`, IP de la VPN…). |
| `-d`, `--directory` | `STORAGE_DIR` | `./uploads` | Carpeta servida. |
| `--token` | `SIMPLEFS_TOKEN` / `SIMPLEFS_TOKEN_FILE` | generado | Token de acceso total. Se guarda en `<directorio>/.simplefs/token`. |
| `--read-token` | `SIMPLEFS_READ_TOKEN` / `_FILE` | — | Token de solo lectura. |
| `--no-auth` | `SIMPLEFS_NO_AUTH=true` | `false` | Desactiva la autenticación (solo detrás de otra capa de acceso). |
| `--allowed-hosts` | `ALLOWED_HOSTS` | — | Nombres de host aceptados (además de IP y `localhost`); `.dominio` admite subdominios, `*` desactiva el control. |
| `--max-upload` | `MAX_UPLOAD_SIZE` | `0` (sin límite) | Tamaño máximo por petición (`512M`, `20G`…). |
| `--file-mode` / `--dir-mode` | `FILE_MODE` / `DIR_MODE` | `0644` / `0755` | Permisos de lo que se crea. |
| `--hide-dotfiles` | `HIDE_DOTFILES=true` | `false` | Oculta y protege los nombres que empiezan por `.`. |
| `--access-log` | `ACCESS_LOG` | `true` | Registro de cada petición (quién, qué, estado, bytes). |

Subcomandos: `simplefs token`, `simplefs healthcheck`, `simplefs version`.

## 🔒 Modelo de seguridad

- Sin token no se puede leer ni escribir nada; el navegador usa una cookie de sesión firmada (`HttpOnly`, `SameSite=Strict`) que no contiene el token. Hay límite de intentos de login.
- Las peticiones con un `Host` que no sea IP, `localhost` o uno de `ALLOWED_HOSTS` se rechazan (DNS rebinding). Las escrituras entre orígenes se rechazan (CSRF).
- Las rutas se resuelven dentro de la carpeta raíz siguiendo symlinks; `.trash` y `.simplefs` están reservados.
- HTML, SVG, XML y cualquier tipo que no sea imagen, audio, vídeo, PDF o texto plano se descargan siempre como adjunto dentro de una CSP `sandbox`.
- Los ZIP se validan antes de extraer (zip slip, symlinks, bombas de compresión, límites de tamaño y número de archivos).
- La interfaz funciona con una CSP sin `unsafe-inline` ni `unsafe-eval`.

## Licencias de terceros

HTMX, highlight.js, Marked, DOMPurify, Tailwind CSS, Inter y JetBrains Mono (OFL) y Material Symbols (Apache 2.0).
