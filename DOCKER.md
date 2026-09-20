# Guía de Docker y Publicación de Paquetes para SimpleFS

Esta guía documenta la contenedorización de **SimpleFS**, su arquitectura de construcción multi-etapa, las opciones de ejecución con Docker y Docker Compose, y la automatización del flujo de CI/CD para publicar la imagen como paquete en **GitHub Container Registry (GHCR)**.

---

## 1. Arquitectura de la Imagen

SimpleFS está diseñado como un único binario ejecutable compilado estáticamente en Go que incrusta todos los assets web (HTML, CSS minificado con Tailwind y JavaScript empaquetado con esbuild).

El archivo [`Dockerfile`](./Dockerfile) utiliza un proceso de compilación **multi-stage** de 3 fases:

1. **`frontend-builder` (Node 20 Alpine)**:
   - Descarga las dependencias con `pnpm` (`--frozen-lockfile`).
   - Compila la suite de fuentes (`Inter`, `Material Symbols Outlined`), las librerías vendor (`htmx`, `marked`, `highlight.js`, `dompurify`) y genera el bundle CSS minificado con Tailwind CSS en `web/static/`.
2. **`backend-builder` (Golang 1.22 Alpine)**:
   - Descarga los módulos de Go.
   - Copia los templates HTML y los assets estáticos generados en la fase previa.
   - Compila un binario estático sin CGO (`CGO_ENABLED=0`) con optimizaciones de tamaño (`-ldflags="-s -w"` y `-trimpath`).
3. **`runtime` (Alpine 3.20)**:
   - Imagen base mínima y endurecida (~15 MB).
   - Paquetes de certificados raíz (`ca-certificates`) y zonas horarias (`tzdata`).
   - Usuario y grupo del sistema sin privilegios `simplefs` (UID/GID `10001`).
   - Directorio de datos `/data` preconfigurado con permisos exclusivos.
   - Punto de montaje declarado como `VOLUME ["/data"]`.

---

## 2. Uso Rápido con Docker

### Descargar y ejecutar desde GitHub Packages (GHCR)

La imagen oficial se publica de forma automática en GitHub Container Registry:

```bash
docker pull ghcr.io/joacohbc/simplefs:latest
```

Para iniciar el contenedor exponiendo el puerto `8080` y persistiendo los archivos en un directorio local:

```bash
docker run -d \
  --name simplefs \
  --restart unless-stopped \
  -p 8080:8080 \
  -v $(pwd)/uploads:/data \
  ghcr.io/joacohbc/simplefs:latest
```

Accede a la interfaz web en: [http://localhost:8080](http://localhost:8080).

---

## 3. Despliegue con Docker Compose

El repositorio incluye un archivo [`docker-compose.yml`](./docker-compose.yml) listo para producción o entornos locales:

```yaml
services:
  simplefs:
    image: ghcr.io/joacohbc/simplefs:latest
    build:
      context: .
      dockerfile: Dockerfile
    container_name: simplefs
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - STORAGE_DIR=/data
    volumes:
      - ./uploads:/data
```

### Iniciar el servicio:
```bash
docker compose up -d
```

### Detener el servicio:
```bash
docker compose down
```

### Ver logs en tiempo real:
```bash
docker compose logs -f simplefs
```

---

## 4. Variables de Entorno y Configuración

| Variable | Valor por Defecto | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto HTTP en el que escucha el servidor. |
| `STORAGE_DIR` | `/data` | Directorio interno donde se guardan y sirven los archivos. |

También puedes pasar banderas CLI al contenedor directamente:

```bash
# Ejemplo: ejecutar en un puerto distinto dentro del contenedor
docker run -d -p 9000:9000 -v $(pwd)/uploads:/data ghcr.io/joacohbc/simplefs:latest -p 9000
```

---

## 5. Construcción Local de la Imagen

Si deseas compilar la imagen localmente desde el código fuente sin depender del registro:

```bash
# Construir la imagen local
docker build -t simplefs:local .

# Ejecutar el contenedor recién compilado
docker run -d -p 8080:8080 -v $(pwd)/uploads:/data simplefs:local
```

---

## 6. Publicación Automatizada como Paquete (CI/CD)

El flujo de trabajo en [`.github/workflows/docker-publish.yml`](./.github/workflows/docker-publish.yml) automatiza la publicación en **GitHub Container Registry (GHCR)**:

- **Plataformas soportadas (Multi-arch)**: `linux/amd64` y `linux/arm64` (Apple Silicon, Raspberry Pi, AWS Graviton).
- **Disparadores**:
  - **Push a `master`**: Compila y publica con las etiquetas `latest`, `master` y el SHA del commit (`sha-<hash>`).
  - **Push de Tags (`v*.*.*`)**: Publica con etiquetas semánticas (`1.0.0`, `1.0`, `1`, `latest`).
  - **Pull Requests**: Ejecuta la compilación de prueba para validar que no haya errores de sintaxis o empaquetado, sin publicar al registro.
  - **Ejecución Manual (`workflow_dispatch`)**: Permite forzar una compilación y publicación desde la interfaz web de GitHub Actions.
- **Autenticación**: Utiliza el token integrado de GitHub (`GITHUB_TOKEN`) con permisos `packages: write`.

### Vinculación de Paquetes en GitHub
Al publicar la primera imagen, GitHub asociará automáticamente el paquete `simplefs` a la pestaña **Packages** del repositorio en:
`https://github.com/Joacohbc/simplefs/pkgs/container/simplefs`
