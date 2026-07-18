# Single-image build: compiles the React frontend, embeds it into the Go
# backend binary, and ships one small runtime image that serves both the API
# and the UI. Build context must be the repository root:
#   docker build -t iss-dashboard .

# --- Stage 1: build the frontend (Vite) ---
FROM node:20-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Same-origin: the built app calls /iss/api/... on whatever host serves it.
ARG VITE_API_BASE_URL=/iss
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

# --- Stage 2: build the backend with the frontend embedded ---
FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Overwrite the committed placeholder with the real Vite build before embedding.
COPY --from=frontend /app/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -o /iss-backend .

# --- Stage 3: runtime ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend /iss-backend /usr/local/bin/iss-backend
RUN mkdir -p /data
EXPOSE 8080
CMD ["iss-backend"]
