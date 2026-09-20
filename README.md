<div align="center">

# SimpleFS

**Un servidor de almacenamiento y exploración de archivos ligero, moderno y ultra rápido para despliegues sin complicaciones.**

</div>

SimpleFS es una solución de gestión y previsualización de archivos autocontenida diseñada para ofrecer el máximo rendimiento con el mínimo consumo de recursos.

## Built With

[![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)](#)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](#)
[![GHCR Package](https://img.shields.io/badge/GitHub_Packages-GHCR-2088FF?logo=github&logoColor=white)](#)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-38B2AC?logo=tailwind-css&logoColor=white)](#)
[![HTML5](https://img.shields.io/badge/HTML5-E34F26?logo=html5&logoColor=white)](#)
[![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?logo=javascript&logoColor=black)](#)
[![GitHub Actions](https://img.shields.io/badge/GitHub_Actions-2088FF?logo=github-actions&logoColor=white)](#)

---

## 🚀 Despliegue Rápido con Docker

La forma recomendada de desplegar SimpleFS es mediante el paquete oficial publicado en **GitHub Container Registry (GHCR)**:

```bash
docker run -d \
  --name simplefs \
  --restart unless-stopped \
  -p 8080:8080 \
  -v $(pwd)/uploads:/data \
  ghcr.io/joacohbc/simplefs:latest
```

Abre tu navegador en [http://localhost:8080](http://localhost:8080) para comenzar a explorar y gestionar archivos.

### Despliegue con Docker Compose

Puedes levantar el servicio en un solo paso utilizando [`docker-compose.yml`](./docker-compose.yml):

```bash
docker compose up -d
```

Para más detalles sobre compilación multi-stage, variables de entorno y configuración de seguridad, consulta la [Guía Completa de Docker](./DOCKER.md).

---

## 🛠️ Desarrollo Local

Requisitos: **Go 1.22+**, **Node.js 20+** y **pnpm 9+**.

```bash
# Instalar dependencias frontend
pnpm install

# Compilar assets y binario
pnpm run build

# Iniciar servidor
./simplefs -p 8080 -d ./uploads
```

### Opciones CLI y Variables de Entorno

| Parámetro | Variable de Entorno | Por Defecto | Descripción |
|---|---|---|---|
| `-p`, `--port` | `PORT` | `8080` | Puerto HTTP del servidor |
| `-d`, `--directory` | `STORAGE_DIR` | `./uploads` | Directorio de almacenamiento de archivos |

