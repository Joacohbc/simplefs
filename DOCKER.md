# Guía de Docker de SimpleFS

## 1. Arquitectura de la imagen

El [`Dockerfile`](./Dockerfile) es multi-etapa y está optimizado para que la imagen sea pequeña y el build rápido:

| Etapa | Base | Qué hace |
|---|---|---|
| `frontend` | `node:24-alpine` (plataforma del host) | Instala dependencias con `pnpm --frozen-lockfile` (con caché de BuildKit) y genera el sprite de iconos, el JS, las fuentes y el CSS. El resultado no depende de la arquitectura, así que se compila **una sola vez**. |
| `backend` | `golang:1.27-alpine` (plataforma del host) | **Compila en cruzado** (`GOOS`/`GOARCH` = destino) un binario estático sin CGO, con `-trimpath -s -w` y la base de zonas horarias embebida (`timetzdata`). No se usa QEMU. |
| `runtime` | `scratch` | Solo el binario (~11 MB) y `/data` (propiedad de UID 10001). Sin shell ni paquetes, así que hay poca superficie de ataque y casi nada que parchear. |

Resultado: imagen de **~11 MB** (antes ~24 MB con Alpine) y builds `linux/amd64` + `linux/arm64` sin emulación.

La imagen incluye un `HEALTHCHECK` (`/simplefs healthcheck`, que no necesita `curl`) y responde a `SIGTERM` con un apagado ordenado: espera hasta 30 s a que terminen las transferencias en curso.

## 2. Ejecutar

```bash
mkdir -p uploads
docker run -d --name simplefs --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  --user "$(id -u):$(id -g)" \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  -e SIMPLEFS_TOKEN="$(openssl rand -hex 24)" \
  -v "$(pwd)/uploads:/data" \
  ghcr.io/joacohbc/simplefs:latest
```

- **`-p 127.0.0.1:8080:8080`**: publícalo solo donde haga falta. Para compartirlo por la VPN, usa la IP de la VPN (`-p 100.64.0.10:8080:8080`). Con `-p 8080:8080` quedaría expuesto en todas las interfaces, porque Docker se salta firewalls como `ufw`.
- **`--user "$(id -u):$(id -g)"`**: los archivos se crean con tu usuario del host. Sin esto pertenecen al UID 10001 y necesitarías `sudo` para leer lo que te envía el agente. Crea `uploads` antes de arrancar para que no lo cree Docker como root.
- **Token**: si no defines `SIMPLEFS_TOKEN`, se genera uno, se imprime en `docker logs simplefs` y se guarda en `uploads/.simplefs/token`. Para verlo: `docker exec simplefs /simplefs token`.

## 3. Docker Compose

[`docker-compose.yml`](./docker-compose.yml) ya trae el endurecimiento (`read_only`, `cap_drop: ALL`, `no-new-privileges`) y toma la configuración de `.env`:

```bash
cp .env.example .env    # ajusta BIND_IP, PUID/PGID, SIMPLEFS_TOKEN, ALLOWED_HOSTS...
mkdir -p uploads
docker compose up -d
docker compose logs -f
```

## 4. Variables de entorno

| Variable | Por defecto | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto dentro del contenedor. |
| `STORAGE_DIR` | `/data` | Carpeta servida. |
| `SIMPLEFS_TOKEN` / `SIMPLEFS_TOKEN_FILE` | generado | Token de acceso total (`_FILE` sirve para Docker secrets). |
| `SIMPLEFS_READ_TOKEN` / `SIMPLEFS_READ_TOKEN_FILE` | — | Token de solo lectura. |
| `ALLOWED_HOSTS` | — | Nombres de host válidos además de IP y `localhost` (p. ej. `pc.tailnet.ts.net`). |
| `MAX_UPLOAD_SIZE` | `0` | Límite por petición (`2G`); `0` = sin límite. |
| `FILE_MODE` / `DIR_MODE` | `0644` / `0755` | Permisos de lo que se crea. |
| `HIDE_DOTFILES` | `false` | Oculta los archivos que empiezan por `.`. |
| `ACCESS_LOG` | `true` | Log de auditoría de cada petición. |
| `SIMPLEFS_NO_AUTH` | `false` | Desactiva la autenticación (solo detrás de otra capa de acceso). |

Los mismos ajustes existen como flags: `docker run ... ghcr.io/joacohbc/simplefs --max-upload 10G`.

## 5. Construir localmente

```bash
docker build -t simplefs:local .
docker buildx build --platform linux/amd64,linux/arm64 -t simplefs:multi .   # multi-arch, sin QEMU
```

## 6. Publicación (CI/CD)

[`.github/workflows/docker-publish.yml`](./.github/workflows/docker-publish.yml):

- **Pull requests**: compila `linux/amd64`, lo carga localmente y lo escanea con **Trivy** (falla si hay vulnerabilidades HIGH/CRITICAL con parche disponible). No publica.
- **Push a `master` / tags `v*.*.*`**: publica en GHCR la imagen multi-arquitectura con SBOM y procedencia (`provenance: mode=max`). Etiquetas: `latest`, rama, `sha-<hash>` y semver (`1.2.3`, `1.2`, `1`).
- **Releases**: GoReleaser publica binarios para Linux, macOS y Windows (amd64/arm64) con `checksums.txt`.
